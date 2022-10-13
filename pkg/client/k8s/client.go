package k8s

import (
	"context"
	"io"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	v1Core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Client struct {
	clientSet kubernetes.Interface
	namespace string
}

func New(namespace string, config *rest.Config) (*Client, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		namespace: namespace,
		clientSet: clientset,
	}, nil
}

func (c *Client) GetDeployment(ctx context.Context, name string) (*v1.Deployment, error) {
	d, err := c.clientSet.AppsV1().Deployments(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return d, nil

}

func (c *Client) GetDeploymentPods(ctx context.Context, d *v1.Deployment) ([]v1Core.Pod, error) {

	targetPods := []v1Core.Pod{}

	rses, err := c.clientSet.AppsV1().ReplicaSets(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	ref, err := NewControllerRef(d)
	if err != nil {
		return nil, err
	}

	var targetRS v1.ReplicaSet

	for _, rs := range rses.Items {
		if HasOwnerReference(&rs, *ref) && HasSameRevision(&rs, d) {
			targetRS = rs
		}
	}

	pods, err := c.clientSet.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	ref, err = NewControllerRef(&targetRS)
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		if HasOwnerReference(&pod, *ref) {
			targetPods = append(targetPods, pod)

		}
	}

	return targetPods, nil

}

func (c *Client) GetObjectEvents(ctx context.Context, object metav1.Object) ([]v1Core.Event, error) {
	output := []v1Core.Event{}

	events, err := c.clientSet.CoreV1().Events(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, event := range events.Items {
		if event.InvolvedObject.UID == object.GetUID() {
			output = append(output, event)
		}

	}

	return output, nil
}

func (c *Client) StreamPodLogs(ctx context.Context, name string) error {
	req := c.clientSet.CoreV1().Pods(c.namespace).GetLogs(name, &v1Core.PodLogOptions{Follow: true})
	readCloser, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer readCloser.Close()

	_, err = io.Copy(log.StandardLogger().Writer(), readCloser)
	return err
}
