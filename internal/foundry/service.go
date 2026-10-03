package foundry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"azfoundrydeck/internal/fault"
)

type Source interface {
	Foundries(context.Context) ([]Foundry, error)
	Deployments(context.Context, Foundry, func(int)) ([]Deployment, error)
}

// sortFoundries orders Foundries by subscription name, then Foundry name, so the first one is stable.
func sortFoundries(foundries []Foundry) {
	slices.SortStableFunc(foundries, func(a, b Foundry) int {
		if c := strings.Compare(a.SubscriptionName, b.SubscriptionName); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
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
		FoundryPhase: "completed", FoundryCount: len(view.Foundries),
		SelectedFoundryName: selected.Name, ModelPhase: "running",
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
	if err := saveModels(modelsPath(file, selected.ID), savedModels{FetchedAt: fetchedAt, Deployments: models}); err != nil {
		return InitialFoundryView{}, err
	}
	view.SelectedFoundryID = selected.ID
	view.Deployments = models
	view.DeploymentsFetchedAt = fetchedAt
	if err := save(file, view); err != nil {
		return InitialFoundryView{}, err
	}
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
	progress := Progress{FoundryPhase: "running", ModelPhase: "waiting"}
	s.emit(ProgressEvent, progress)
	source, err := s.source()
	if err != nil {
		return InitialFoundryView{}, err
	}
	foundries, err := source.Foundries(ctx)
	if err != nil {
		return InitialFoundryView{}, err
	}
	sortFoundries(foundries)
	progress.FoundryPhase = "completed"
	progress.FoundryCount = len(foundries)
	s.emit(ProgressEvent, progress)
	view.Foundries = foundries
	view.FoundriesFetchedAt = fetchedNow()
	listed := slices.ContainsFunc(foundries, func(foundry Foundry) bool { return foundry.ID == view.SelectedFoundryID })
	if len(foundries) == 0 {
		// No Foundry is a normal result: nothing is selected and no models are fetched.
		view.Foundries, view.SelectedFoundryID = []Foundry{}, ""
		view.Deployments, view.DeploymentsFetchedAt = []Deployment{}, ""
	} else if !listed {
		selected := foundries[0]
		progress.SelectedFoundryName = selected.Name
		progress.ModelPhase = "running"
		s.emit(ProgressEvent, progress)
		models, err := source.Deployments(ctx, selected, func(count int) {
			progress.ModelCount = count
			s.emit(ProgressEvent, progress)
		})
		if err != nil {
			return InitialFoundryView{}, err
		}
		progress.ModelPhase = "completed"
		progress.ModelCount = len(models)
		s.emit(ProgressEvent, progress)
		view.SelectedFoundryID = selected.ID
		view.Deployments = models
		view.DeploymentsFetchedAt = fetchedNow()
		if err := saveModels(modelsPath(file, selected.ID), savedModels{FetchedAt: view.DeploymentsFetchedAt, Deployments: view.Deployments}); err != nil {
			return InitialFoundryView{}, err
		}
	}
	if err := removeModelsExcept(file, foundries); err != nil {
		return InitialFoundryView{}, err
	}
	if err := save(file, view); err != nil {
		return InitialFoundryView{}, err
	}
	return view, nil
}

func (s *Service) acquire(ctx context.Context, file string) (InitialFoundryView, error) {
	progress := Progress{FoundryPhase: "running", ModelPhase: "waiting"}
	s.emit(ProgressEvent, progress)
	source, err := s.source()
	if err != nil {
		return InitialFoundryView{}, err
	}
	foundries, err := source.Foundries(ctx)
	if err != nil {
		return InitialFoundryView{}, err
	}
	sortFoundries(foundries)
	progress.FoundryPhase = "completed"
	progress.FoundryCount = len(foundries)
	s.emit(ProgressEvent, progress)
	view := InitialFoundryView{Foundries: foundries, FoundriesFetchedAt: fetchedNow()}
	// No Foundry is a normal result: only the empty list and its fetch time are saved.
	if len(foundries) == 0 {
		view.Foundries, view.Deployments = []Foundry{}, []Deployment{}
	} else {
		selected := foundries[0]
		view.SelectedFoundryID = selected.ID
		progress.SelectedFoundryName = selected.Name
		progress.ModelPhase = "running"
		s.emit(ProgressEvent, progress)
		models, err := source.Deployments(ctx, selected, func(count int) {
			progress.ModelCount = count
			s.emit(ProgressEvent, progress)
		})
		if err != nil {
			return InitialFoundryView{}, err
		}
		progress.ModelPhase = "completed"
		progress.ModelCount = len(models)
		s.emit(ProgressEvent, progress)
		view.Deployments = models
		view.DeploymentsFetchedAt = fetchedNow()
		if err := saveModels(modelsPath(file, selected.ID), savedModels{FetchedAt: view.DeploymentsFetchedAt, Deployments: models}); err != nil {
			return InitialFoundryView{}, err
		}
	}
	if err := save(file, view); err != nil {
		return InitialFoundryView{}, err
	}
	return view, nil
}
