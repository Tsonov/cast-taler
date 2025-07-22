package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetNodeZone(ctx context.Context, cl client.Client, nodeName string) (string, error) {
	node := &corev1.Node{}
	if err := cl.Get(ctx, client.ObjectKey{Name: nodeName}, node); err != nil {
		return "", fmt.Errorf("failed to get node %q: %w", nodeName, err)
	}

	if node.Labels == nil {
		return "", nil // No labels present
	}

	return node.Labels[corev1.LabelTopologyZone], nil
}
