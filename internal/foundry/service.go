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
	// Held in memory only and guarded by operations. The deployments are those of
	// the selected Foundry as last fetched; the limits and the connection are
	// discarded whenever the view is loaded or the Foundry changes.
	current    deploymentsCache
	limits     limitsCache
	connection connectionCache
}

type deploymentsCache struct {
	file, foundryID string
	deployments     []Deployment
	fetchedAt       string
}

type limitsCache struct {
	file, foundryID string
	value           CapacityLimits
	fetch           *limitsFetch
}

// readView reads the saved Foundry list and selection and adds the deployments held in memory.
func (s *Service) readView(file string) (InitialFoundryView, error) {
	view, err := read(file)
	if err != nil {
		return view, err
	}
	if view.SelectedFoundryID != "" && s.current.file == file && s.current.foundryID == view.SelectedFoundryID {
		view.Deployments, view.DeploymentsFetchedAt = s.current.deployments, s.current.fetchedAt
	}
	return view, nil
}

func (s *Service) setDeployments(file, foundryID string, deployments []Deployment, fetchedAt string) {
	s.current = deploymentsCache{file: file, foundryID: foundryID, deployments: deployments, fetchedAt: fetchedAt}
}

func (s *Service) clearDeployments() {
	if s.limits.fetch != nil {
		s.limits.fetch.cancel()
	}
	if s.connection.fetch != nil {
		s.connection.fetch.cancel()
	}
	s.current, s.limits, s.connection = deploymentsCache{}, limitsCache{}, connectionCache{}
}

// StopView is called with the shared operation lock held by the auth service.
// Waiting for the cancelled fetch prevents it from writing a token cache after
// logout has deleted that cache. This package function is not a Wails binding.
func StopView(s *Service, discard bool) {
	if s.limits.fetch != nil {
		s.limits.fetch.cancel()
		<-s.limits.fetch.done
	}
	if s.connection.fetch != nil {
		s.connection.fetch.cancel()
		<-s.connection.fetch.done
	}
	if discard {
		s.clearDeployments()
	} else if s.limits.fetch != nil {
		// A failed logout keeps the screen mounted. Let it observe the stopped
		// fetch instead of retaining Loading indefinitely.
		s.emit(CapacityReadyEvent, s.limits.foundryID)
	}
	if !discard && s.connection.fetch != nil {
		s.emit(ConnectionReadyEvent, s.connection.foundryID)
	}
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

// load restores the saved Foundry list and selection, then fetches the selected
// Foundry's deployments from Azure. Without saved data it also fetches the Foundries.
func (s *Service) load(ctx context.Context, file string) (InitialFoundryView, error) {
	s.clearDeployments()
	view, err := read(file)
	if errors.Is(err, os.ErrNotExist) {
		return s.acquire(ctx, file)
	}
	if err != nil {
		return InitialFoundryView{}, err
	}
	index := slices.IndexFunc(view.Foundries, func(foundry Foundry) bool { return foundry.ID == view.SelectedFoundryID })
	if index < 0 {
		// A saved list without Foundries has nothing to select or fetch.
		return view, nil
	}
	return s.acquireModels(ctx, file, view, view.Foundries[index])
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
	view, err := s.readView(file)
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
	s.clearDeployments()
	return s.acquireModels(ctx, file, view, selected)
}

// acquireModels fetches all models of the Foundry from Azure, keeps them in
// memory and saves the state with that Foundry selected.
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
	s.startCapacityLimits(ctx, file, selected, source)
	s.startConnection(ctx, file, selected, source)
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
	view.SelectedFoundryID = selected.ID
	view.Deployments = models
	view.DeploymentsFetchedAt = fetchedAt
	if err := save(file, view); err != nil {
		return InitialFoundryView{}, err
	}
	s.setDeployments(file, selected.ID, models, fetchedAt)
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

// refreshDeployments re-fetches the selected Foundry's models from Azure.
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
	view, err := s.readView(file)
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
	previous := view.SelectedFoundryID
	view.Foundries = foundries
	view.FoundriesFetchedAt = fetchedNow()
	if len(foundries) == 0 {
		// No Foundry is a normal result: nothing is selected and no models are fetched.
		view.Foundries, view.SelectedFoundryID = []Foundry{}, ""
		view.Deployments, view.DeploymentsFetchedAt = []Deployment{}, ""
		s.clearDeployments()
	} else if !slices.ContainsFunc(foundries, func(foundry Foundry) bool { return foundry.ID == previous }) {
		s.clearDeployments()
		return s.acquireModels(ctx, file, view, foundries[0])
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
	view := InitialFoundryView{Foundries: foundries, Deployments: []Deployment{}, FoundriesFetchedAt: fetchedNow()}
	// No Foundry is a normal result: only the empty list and its fetch time are saved.
	if len(foundries) == 0 {
		view.Foundries = []Foundry{}
		if err := save(file, view); err != nil {
			return InitialFoundryView{}, err
		}
		return view, nil
	}
	return s.acquireModels(ctx, file, view, foundries[0])
}
