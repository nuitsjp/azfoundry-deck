package service

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestDeploymentServiceNormalizesArraysAndReportsEnvironment(t *testing.T) {
	var progress []DeploymentProgress
	serviceUnderTest := NewDeploymentService(func(context.Context, func(DeploymentProgress)) (DeploymentResult, error) {
		return DeploymentResult{}, nil
	}, true, func(event DeploymentProgress) { progress = append(progress, event) })

	result, err := serviceUnderTest.GetDeployments(context.Background(), "request-1")
	if err != nil {
		t.Fatalf("GetDeployments returned error: %v", err)
	}
	if result.Deployments == nil || result.Failures == nil {
		t.Fatal("result arrays must be empty arrays, not nil")
	}
	if !serviceUnderTest.GetEnvironment().Mock {
		t.Fatal("GetEnvironment().Mock = false, want true")
	}
	if len(progress) != 0 {
		t.Fatalf("progress count = %d, want 0", len(progress))
	}
}

func TestDeploymentServicePassesContextAndErrorThrough(t *testing.T) {
	wantErr := errors.New("fetch failed")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	serviceUnderTest := NewDeploymentService(func(got context.Context, _ func(DeploymentProgress)) (DeploymentResult, error) {
		if got != ctx {
			t.Fatal("GetDeployments did not pass the caller context")
		}
		return DeploymentResult{}, wantErr
	}, false, nil)

	_, err := serviceUnderTest.GetDeployments(ctx, "request-1")
	if !errors.Is(err, wantErr) {
		t.Fatalf("GetDeployments error = %v, want %v", err, wantErr)
	}
	if serviceUnderTest.GetEnvironment().Mock {
		t.Fatal("GetEnvironment().Mock = true, want false")
	}
}

func TestDeploymentServiceAddsRequestMetadataAndNormalizesSnapshots(t *testing.T) {
	var emitted []DeploymentProgress
	serviceUnderTest := NewDeploymentService(func(_ context.Context, report func(DeploymentProgress)) (DeploymentResult, error) {
		report(DeploymentProgress{
			Stage: DeploymentProgressStageFetching,
			Result: DeploymentResult{
				Deployments: []Deployment{{Name: "first"}},
				Failures:    []FetchFailure{},
			},
		})
		report(DeploymentProgress{
			Stage:  DeploymentProgressStageFetching,
			Result: DeploymentResult{Deployments: []Deployment{{Name: "second"}}, Failures: []FetchFailure{}},
		})
		return DeploymentResult{Deployments: []Deployment{{Name: "second"}}, Failures: []FetchFailure{}}, nil
	}, false, func(event DeploymentProgress) { emitted = append(emitted, event) })

	result, err := serviceUnderTest.GetDeployments(context.Background(), "request-42")
	if err != nil {
		t.Fatalf("GetDeployments returned error: %v", err)
	}
	if len(emitted) != 2 {
		t.Fatalf("progress count = %d, want 2", len(emitted))
	}
	for index, event := range emitted {
		if event.RequestID != "request-42" {
			t.Errorf("event %d request ID = %q, want request-42", index, event.RequestID)
		}
		if event.Sequence != uint64(index+1) {
			t.Errorf("event %d sequence = %d, want %d", index, event.Sequence, index+1)
		}
		if event.Result.Deployments == nil || event.Result.Failures == nil {
			t.Errorf("event %d result arrays must be non-nil", index)
		}
	}
	if emitted[0].Result.Deployments[0].Name != "first" {
		t.Fatalf("first snapshot deployment = %q, want first", emitted[0].Result.Deployments[0].Name)
	}
	if emitted[1].Result.Deployments[0].Name != "second" {
		t.Fatalf("second snapshot deployment = %q, want second", emitted[1].Result.Deployments[0].Name)
	}
	if result.Deployments[0].Name != "second" {
		t.Fatalf("final deployment = %q, want second", result.Deployments[0].Name)
	}
}

func TestDeploymentServiceSequencesAreIndependentPerRequest(t *testing.T) {
	var mu sync.Mutex
	emitted := make([]DeploymentProgress, 0, 4)
	serviceUnderTest := NewDeploymentService(func(_ context.Context, report func(DeploymentProgress)) (DeploymentResult, error) {
		report(DeploymentProgress{Stage: DeploymentProgressStageDiscovering, Result: DeploymentResult{}})
		report(DeploymentProgress{Stage: DeploymentProgressStageFetching, Result: DeploymentResult{}})
		return DeploymentResult{}, nil
	}, false, func(event DeploymentProgress) {
		mu.Lock()
		emitted = append(emitted, event)
		mu.Unlock()
	})

	var wait sync.WaitGroup
	wait.Add(2)
	for _, requestID := range []string{"request-a", "request-b"} {
		go func(requestID string) {
			defer wait.Done()
			if _, err := serviceUnderTest.GetDeployments(context.Background(), requestID); err != nil {
				t.Errorf("GetDeployments(%q) returned error: %v", requestID, err)
			}
		}(requestID)
	}
	wait.Wait()

	byRequest := map[string][]uint64{}
	for _, event := range emitted {
		byRequest[event.RequestID] = append(byRequest[event.RequestID], event.Sequence)
	}
	for _, requestID := range []string{"request-a", "request-b"} {
		sequences := byRequest[requestID]
		if len(sequences) != 2 || sequences[0] != 1 || sequences[1] != 2 {
			t.Errorf("%s sequences = %v, want [1 2]", requestID, sequences)
		}
	}
}
