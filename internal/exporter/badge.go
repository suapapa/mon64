package exporter

import (
	"fmt"
	"image"

	"github.com/suapapa/mon64/internal/config"
	"github.com/suapapa/mon64/internal/domain"
)

// BadgePNG renders a named badge by type over the given node states (badge order).
func BadgePNG(badgeType string, nodes []domain.NodeState) ([]byte, error) {
	img, err := BadgeImage(badgeType, nodes)
	if err != nil {
		return nil, err
	}
	return encodePNG(img)
}

// BadgeImage renders a named badge by type over the given node states (badge order).
func BadgeImage(badgeType string, nodes []domain.NodeState) (image.Image, error) {
	switch badgeType {
	case config.BadgeTypeRect64:
		return rect64Image(nodes), nil
	case config.BadgeTypeCircle128:
		return nil, fmt.Errorf("badge type %q is not implemented yet", badgeType)
	default:
		return nil, fmt.Errorf("unknown badge type %q", badgeType)
	}
}

// BadgeSize returns the PNG pixel size for a badge type and node list.
func BadgeSize(badgeType string, nodes []domain.NodeState) (w, h int, err error) {
	switch badgeType {
	case config.BadgeTypeRect64:
		height := rect64Height(nodes)
		if height == 0 {
			height = 1
		}
		return badgeWidth, height, nil
	case config.BadgeTypeCircle128:
		return 0, 0, fmt.Errorf("badge type %q is not implemented yet", badgeType)
	default:
		return 0, 0, fmt.Errorf("unknown badge type %q", badgeType)
	}
}

// SelectBadgeNodes returns snapshot nodes in badge.Nodes order.
// Unknown names are skipped (config validation normally prevents this).
func SelectBadgeNodes(badge config.BadgeConfig, snap domain.Snapshot) []domain.NodeState {
	byName := make(map[string]domain.NodeState, len(snap.Nodes))
	for _, n := range snap.Nodes {
		byName[n.Name] = n
	}
	out := make([]domain.NodeState, 0, len(badge.Nodes))
	for _, name := range badge.Nodes {
		if n, ok := byName[name]; ok {
			out = append(out, n)
		}
	}
	return out
}
