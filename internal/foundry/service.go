package foundry

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"azfoundrydeck/internal/fault"
	"golang.org/x/sync/errgroup"
)

type Source interface {
	Discover(context.Context, func(Foundry), func(DiscoveryProgress)) ([]Foundry, error)
	Deployments(context.Context, Foundry, func(int)) ([]Deployment, error)
}

type firstFoundry struct {
	foundry Foundry
	err     error
}

type Service struct {
	source   func() (Source, error)
	signedIn func(context.Context) error
	file     string
	logger   *slog.Logger
	emit     func(string, any)
}

func New(source func() (Source, error), signedIn func(context.Context) error, file string, logger *slog.Logger, emit func(string, any)) *Service {
	return &Service{source: source, signedIn: signedIn, file: file, logger: logger, emit: emit}
}

func (s *Service) GetInitialView(ctx context.Context) (InitialFoundryView, error) {
	if err := s.signedIn(ctx); err != nil {
		return InitialFoundryView{}, err
	}
	view, err := s.load(ctx)
	if err != nil {
		s.logger.Error("operation_failed", "operation", "foundry.GetInitialView", "cause", err)
		if ctx.Err() != nil {
			return InitialFoundryView{}, fault.Public(ctx.Err())
		}
		return InitialFoundryView{}, fault.New("FOUNDRY_LOAD_FAILED", "Foundry・デプロイモデルの取得またはファイル保存に失敗しました。")
	}
	return view, nil
}

func (s *Service) load(ctx context.Context) (InitialFoundryView, error) {
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
		once.Do(func() {
			if err == nil {
				err = fmt.Errorf("no Foundry was discovered")
			}
			first <- firstFoundry{err: err}
		})
		view.Foundries = foundries
		return err
	})
	group.Go(func() error {
		select {
		case result := <-first:
			if result.err != nil {
				return result.err
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
	if err := save(s.file, view); err != nil {
		return InitialFoundryView{}, err
	}
	update(func(p *Progress) { p.SavePhase = "completed" })
	return view, nil
}
