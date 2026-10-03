package foundry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sync"
	"time"

	"azfoundrydeck/internal/fault"
	"golang.org/x/sync/errgroup"
)

type Source interface {
	Discover(context.Context, func(Foundry), func(DiscoveryProgress)) ([]Foundry, error)
	Deployments(context.Context, Foundry, func(int)) ([]Deployment, error)
}

// firstFoundry is the first discovered Foundry, or the end of discovery with none (err nil, none true).
type firstFoundry struct {
	foundry Foundry
	none    bool
	err     error
}

type Service struct {
	source     func() (Source, error)
	signedIn   func(context.Context) error
	file       func() (string, error)
	logger     *slog.Logger
	emit       func(string, any)
	operations *sync.Mutex
}

func New(operations *sync.Mutex, source func() (Source, error), signedIn func(context.Context) error, file func() (string, error), logger *slog.Logger, emit func(string, any)) *Service {
	return &Service{source: source, signedIn: signedIn, file: file, logger: logger, emit: emit, operations: operations}
}

func (s *Service) GetInitialView(ctx context.Context) (InitialFoundryView, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return InitialFoundryView{}, err
	}
	file, err := s.file()
	var view InitialFoundryView
	if err == nil {
		view, err = s.load(ctx, file)
	}
	if err != nil {
		s.logger.Error("operation_failed", "operation", "foundry.GetInitialView", "cause", err)
		if ctx.Err() != nil {
			return InitialFoundryView{}, fault.Public(ctx.Err())
		}
		return InitialFoundryView{}, fault.New("FOUNDRY_LOAD_FAILED", "Foundry・デプロイモデルの取得またはファイルの読み込み・保存に失敗しました。")
	}
	return view, nil
}

func fetchedNow() string {
	return time.Now().Format(time.RFC3339)
}

func (s *Service) load(ctx context.Context, file string) (InitialFoundryView, error) {
	view, err := read(file)
	if !errors.Is(err, os.ErrNotExist) {
		return view, err
	}
	return s.acquire(ctx, file)
}

func (s *Service) ChangeFoundry(ctx context.Context, id string) (InitialFoundryView, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return InitialFoundryView{}, err
	}
	file, err := s.file()
	var view InitialFoundryView
	if err == nil {
		view, err = s.change(ctx, file, id)
	}
	if err != nil {
		s.logger.Error("operation_failed", "operation", "foundry.ChangeFoundry", "cause", err)
		if ctx.Err() != nil {
			return InitialFoundryView{}, fault.Public(ctx.Err())
		}
		return InitialFoundryView{}, fault.New("FOUNDRY_LOAD_FAILED", "Foundry・デプロイモデルの取得またはファイルの読み込み・保存に失敗しました。")
	}
	return view, nil
}

func (s *Service) change(ctx context.Context, file, id string) (InitialFoundryView, error) {
	view, err := read(file)
	if err != nil {
		return InitialFoundryView{}, err
	}
	var selected Foundry
	for _, foundry := range view.Foundries {
		if foundry.ID == id {
			selected = foundry
			break
		}
	}
	if selected.ID == "" {
		return InitialFoundryView{}, fmt.Errorf("selected Foundry is not in the saved list")
	}
	if id == view.SelectedFoundryID {
		return view, nil
	}
	oldPath := modelsPath(file, view.SelectedFoundryID)
	if _, err := os.Stat(oldPath); errors.Is(err, os.ErrNotExist) {
		if err := saveModels(oldPath, savedModels{FetchedAt: view.DeploymentsFetchedAt, Deployments: view.Deployments}); err != nil {
			return InitialFoundryView{}, err
		}
	} else if err != nil {
		return InitialFoundryView{}, err
	}
	targetPath := modelsPath(file, id)
	saved, err := readModels(targetPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return InitialFoundryView{}, err
	}
	if errors.Is(err, os.ErrNotExist) {
		return s.acquireModels(ctx, file, view, selected)
	}
	view.SelectedFoundryID = id
	view.Deployments = saved.Deployments
	view.DeploymentsFetchedAt = saved.FetchedAt
	if err := save(file, view); err != nil {
		return InitialFoundryView{}, err
	}
	return view, nil
}

// acquireModels fetches all models of the Foundry from Azure, saves its model
// file and then the state with that Foundry selected.
func (s *Service) acquireModels(ctx context.Context, file string, view InitialFoundryView, selected Foundry) (InitialFoundryView, error) {
	progress := Progress{
		SubscriptionSearch: "completed", Subscriptions: []SubscriptionProgress{},
		SelectedFoundryName: selected.Name, ModelPhase: "running", SavePhase: "waiting",
	}
	s.emit(ProgressEvent, progress)
	source, err := s.source()
	if err != nil {
		return InitialFoundryView{}, err
	}
	models, err := source.Deployments(ctx, selected, func(count int) {
		progress.ModelCount = count
		s.emit(ProgressEvent, progress)
	})
	if err != nil {
		return InitialFoundryView{}, err
	}
	fetchedAt := fetchedNow()
	progress.ModelPhase = "completed"
	progress.ModelCount = len(models)
	s.emit(ProgressEvent, progress)
	progress.SavePhase = "running"
	s.emit(ProgressEvent, progress)
	if err := saveModels(modelsPath(file, selected.ID), savedModels{FetchedAt: fetchedAt, Deployments: models}); err != nil {
		return InitialFoundryView{}, err
	}
	view.SelectedFoundryID = selected.ID
	view.Deployments = models
	view.DeploymentsFetchedAt = fetchedAt
	if err := save(file, view); err != nil {
		return InitialFoundryView{}, err
	}
	progress.SavePhase = "completed"
	s.emit(ProgressEvent, progress)
	return view, nil
}

func (s *Service) RefreshDeployments(ctx context.Context) (InitialFoundryView, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return InitialFoundryView{}, err
	}
	file, err := s.file()
	var view InitialFoundryView
	if err == nil {
		view, err = s.refreshDeployments(ctx, file)
	}
	if err != nil {
		s.logger.Error("operation_failed", "operation", "foundry.RefreshDeployments", "cause", err)
		if ctx.Err() != nil {
			return InitialFoundryView{}, fault.Public(ctx.Err())
		}
		return InitialFoundryView{}, fault.New("FOUNDRY_LOAD_FAILED", "Foundry・デプロイモデルの取得またはファイルの読み込み・保存に失敗しました。")
	}
	return view, nil
}

// refreshDeployments re-fetches the selected Foundry's models regardless of its saved file.
func (s *Service) refreshDeployments(ctx context.Context, file string) (InitialFoundryView, error) {
	view, err := read(file)
	if err != nil {
		return InitialFoundryView{}, err
	}
	index := slices.IndexFunc(view.Foundries, func(foundry Foundry) bool { return foundry.ID == view.SelectedFoundryID })
	if index < 0 {
		return InitialFoundryView{}, fmt.Errorf("selected Foundry is not in the saved list")
	}
	return s.acquireModels(ctx, file, view, view.Foundries[index])
}

func (s *Service) RefreshFoundries(ctx context.Context) (InitialFoundryView, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if err := s.signedIn(ctx); err != nil {
		return InitialFoundryView{}, err
	}
	file, err := s.file()
	var view InitialFoundryView
	if err == nil {
		view, err = s.refresh(ctx, file)
	}
	if err != nil {
		s.logger.Error("operation_failed", "operation", "foundry.RefreshFoundries", "cause", err)
		if ctx.Err() != nil {
			return InitialFoundryView{}, fault.Public(ctx.Err())
		}
		return InitialFoundryView{}, fault.New("FOUNDRY_LOAD_FAILED", "Foundry・デプロイモデルの取得またはファイルの読み込み・保存に失敗しました。")
	}
	return view, nil
}

// refresh re-fetches the Foundry list. It fetches models only when the selected
// Foundry is no longer listed, and then selects the first Foundry.
func (s *Service) refresh(ctx context.Context, file string) (InitialFoundryView, error) {
	view, err := read(file)
	if err != nil {
		return InitialFoundryView{}, err
	}
	progress := Progress{
		SubscriptionSearch: "searching", Subscriptions: []SubscriptionProgress{},
		ModelPhase: "waiting", SavePhase: "waiting",
	}
	var progressMu sync.Mutex
	update := func(change func(*Progress)) {
		progressMu.Lock()
		defer progressMu.Unlock()
		change(&progress)
		s.emit(ProgressEvent, progress)
	}
	update(func(*Progress) {})
	source, err := s.source()
	if err != nil {
		return InitialFoundryView{}, err
	}
	foundries, err := source.Discover(ctx, func(Foundry) {}, func(discovery DiscoveryProgress) {
		update(func(p *Progress) {
			p.SubscriptionSearch = discovery.SubscriptionSearch
			p.Subscriptions = discovery.Subscriptions
		})
	})
	if err != nil {
		return InitialFoundryView{}, err
	}
	if len(foundries) == 0 {
		return InitialFoundryView{}, fmt.Errorf("no Foundry was discovered")
	}
	view.Foundries = foundries
	view.FoundriesFetchedAt = fetchedNow()
	listed := slices.ContainsFunc(foundries, func(foundry Foundry) bool { return foundry.ID == view.SelectedFoundryID })
	if !listed {
		selected := foundries[0]
		update(func(p *Progress) {
			p.SelectedFoundryName = selected.Name
			p.ModelPhase = "running"
		})
		models, err := source.Deployments(ctx, selected, func(count int) {
			update(func(p *Progress) { p.ModelCount = count })
		})
		if err != nil {
			return InitialFoundryView{}, err
		}
		update(func(p *Progress) {
			p.ModelPhase = "completed"
			p.ModelCount = len(models)
		})
		view.SelectedFoundryID = selected.ID
		view.Deployments = models
		view.DeploymentsFetchedAt = fetchedNow()
	}
	update(func(p *Progress) { p.SavePhase = "running" })
	if !listed {
		if err := saveModels(modelsPath(file, view.SelectedFoundryID), savedModels{FetchedAt: view.DeploymentsFetchedAt, Deployments: view.Deployments}); err != nil {
			return InitialFoundryView{}, err
		}
	}
	if err := removeModelsExcept(file, foundries); err != nil {
		return InitialFoundryView{}, err
	}
	if err := save(file, view); err != nil {
		return InitialFoundryView{}, err
	}
	update(func(p *Progress) { p.SavePhase = "completed" })
	return view, nil
}

func (s *Service) acquire(ctx context.Context, file string) (InitialFoundryView, error) {
	progress := Progress{
		SubscriptionSearch: "searching", Subscriptions: []SubscriptionProgress{},
		ModelPhase: "waiting", SavePhase: "waiting",
	}
	var progressMu sync.Mutex
	update := func(change func(*Progress)) {
		progressMu.Lock()
		defer progressMu.Unlock()
		change(&progress)
		s.emit(ProgressEvent, progress)
	}
	update(func(*Progress) {})
	source, err := s.source()
	if err != nil {
		return InitialFoundryView{}, err
	}
	group, workCtx := errgroup.WithContext(ctx)
	first := make(chan firstFoundry, 1)
	var once sync.Once
	var view InitialFoundryView
	group.Go(func() error {
		foundries, err := source.Discover(workCtx, func(foundry Foundry) {
			once.Do(func() { first <- firstFoundry{foundry: foundry} })
		}, func(discovery DiscoveryProgress) {
			update(func(p *Progress) {
				p.SubscriptionSearch = discovery.SubscriptionSearch
				p.Subscriptions = discovery.Subscriptions
			})
		})
		once.Do(func() { first <- firstFoundry{none: err == nil, err: err} })
		view.Foundries = foundries
		view.FoundriesFetchedAt = fetchedNow()
		return err
	})
	group.Go(func() error {
		select {
		case result := <-first:
			if result.err != nil {
				return result.err
			}
			if result.none {
				return nil
			}
			foundry := result.foundry
			view.SelectedFoundryID = foundry.ID
			update(func(p *Progress) {
				p.SelectedFoundryName = foundry.Name
				p.ModelPhase = "running"
			})
			models, err := source.Deployments(workCtx, foundry, func(count int) {
				update(func(p *Progress) { p.ModelCount = count })
			})
			view.Deployments = models
			view.DeploymentsFetchedAt = fetchedNow()
			if err == nil {
				update(func(p *Progress) {
					p.ModelPhase = "completed"
					p.ModelCount = len(models)
				})
			}
			return err
		case <-workCtx.Done():
			return workCtx.Err()
		}
	})
	if err := group.Wait(); err != nil {
		return InitialFoundryView{}, err
	}
	update(func(p *Progress) { p.SavePhase = "running" })
	// No Foundry is a normal result: only the empty list and its fetch time are saved.
	if len(view.Foundries) == 0 {
		view.Foundries, view.Deployments = []Foundry{}, []Deployment{}
	} else if err := saveModels(modelsPath(file, view.SelectedFoundryID), savedModels{FetchedAt: view.DeploymentsFetchedAt, Deployments: view.Deployments}); err != nil {
		return InitialFoundryView{}, err
	}
	if err := save(file, view); err != nil {
		return InitialFoundryView{}, err
	}
	update(func(p *Progress) { p.SavePhase = "completed" })
	return view, nil
}
