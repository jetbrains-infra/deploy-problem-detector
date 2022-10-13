package problemdetector

import (
	"context"
	"fmt"

	"github.com/jetbrains-infra/deploy-problem-detector/pkg/client/k8s"
	"github.com/pkg/errors"
	v1 "k8s.io/api/apps/v1"
	v1Core "k8s.io/api/core/v1"
)

const (
	ReasonTimedOutReason     = "ProgressDeadlineExceeded"
	ReasonFailedCreate       = "FailedCreate"
	ReasonContainersNotReady = "ContainersNotReady"
	ReasonUnhealthy          = "Unhealthy"
)

var ErrPodIsNotReady = errors.New("Pod is not ready")
var ErrWaitingForContainerToStart = errors.New("Waiting container")

type ProblemDetector struct {
	client       *k8s.Client
	streamedPods map[string]bool
}

func New(client *k8s.Client) *ProblemDetector {
	return &ProblemDetector{
		client:       client,
		streamedPods: map[string]bool{},
	}
}

func (pd *ProblemDetector) RolloutComplete(dep *v1.Deployment) bool {
	requiredReplicas := *dep.Spec.Replicas
	statusReplicas := dep.Status.Replicas
	availableReplicas := dep.Status.AvailableReplicas
	updatedReplicas := dep.Status.UpdatedReplicas

	if requiredReplicas == statusReplicas &&
		requiredReplicas == availableReplicas &&
		requiredReplicas == updatedReplicas {
		return true
	}

	return false
}

func (pd *ProblemDetector) QuotaProblem(dep *v1.Deployment) error {
	for _, cond := range dep.Status.Conditions {
		if cond.Reason == ReasonFailedCreate {
			return fmt.Errorf(cond.Message)
		}
	}

	return nil
}

func (pd *ProblemDetector) ReadinessProblem(ctx context.Context, dep *v1.Deployment) error {
	pods, err := pd.client.GetDeploymentPods(ctx, dep)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		for _, cond := range pod.Status.Conditions {
			if cond.Reason == ReasonContainersNotReady {
				events, err := pd.client.GetObjectEvents(ctx, &pod)
				if err != nil {
					return err
				}
				for _, ev := range events {
					if ev.Type == v1Core.EventTypeWarning && ev.Reason == ReasonUnhealthy {
						return errors.Wrap(ErrPodIsNotReady, fmt.Sprintf("%s: %s", ev.Message, cond.Message))
					}
				}
				return errors.Wrap(ErrPodIsNotReady, cond.Message)
			}
		}
	}

	return nil
}

func (pd *ProblemDetector) StreamLogs(ctx context.Context, dep *v1.Deployment) error {
	pods, err := pd.client.GetDeploymentPods(ctx, dep)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		_, ok := pd.streamedPods[pod.GetName()]
		if !ok {
			go func(ctx context.Context, pod *v1Core.Pod, streamedPods map[string]bool) {
				if err := pd.client.StreamPodLogs(ctx, pod.GetName()); err != nil {
				} else {
					pd.streamedPods[pod.GetName()] = true
				}
			}(ctx, &pod, pd.streamedPods)
		}
	}
	return nil
}

func (pd *ProblemDetector) ContainersStartProblems(ctx context.Context, dep *v1.Deployment) error {
	pods, err := pd.client.GetDeploymentPods(ctx, dep)
	if err != nil {
		return err
	}
	for _, pod := range pods {
		for _, cond := range pod.Status.ContainerStatuses {
			if !cond.Ready && cond.State.Waiting != nil {
				return errors.Wrap(ErrWaitingForContainerToStart,
					fmt.Sprintf("%s: %s", cond.State.Waiting.Reason, cond.State.Waiting.Message))
			}
		}
	}

	return nil
}

func (pd *ProblemDetector) DeployTimeout(dep *v1.Deployment) bool {
	for _, cond := range dep.Status.Conditions {
		if cond.Reason == ReasonTimedOutReason {
			return true
		}
	}
	return false
}
