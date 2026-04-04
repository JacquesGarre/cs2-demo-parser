package processing

import (
	"embed"
	"encoding/json"
	"math"
	"strings"
	"sync"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
)

//go:embed callouts/*.json
var embeddedCalloutFiles embed.FS

const calloutSnapDistance = 160.0
const calloutVerticalTolerance = 96.0

type calloutFile struct {
	Map      string        `json:"map"`
	Callouts []calloutArea `json:"callouts"`
}

type calloutArea struct {
	Name    string  `json:"name"`
	OriginX float64 `json:"origin_x"`
	OriginY float64 `json:"origin_y"`
	OriginZ float64 `json:"origin_z"`
	MinX    float64 `json:"min_x"`
	MinY    float64 `json:"min_y"`
	MinZ    float64 `json:"min_z"`
	MaxX    float64 `json:"max_x"`
	MaxY    float64 `json:"max_y"`
	MaxZ    float64 `json:"max_z"`
}

type calloutResolver struct {
	areas []calloutArea
}

var calloutResolverCache sync.Map

func loadCalloutResolver(mapName string) *calloutResolver {
	key := normalizeCalloutMapKey(mapName)
	if key == "" {
		return &calloutResolver{}
	}

	if cached, ok := calloutResolverCache.Load(key); ok {
		if resolver, ok := cached.(*calloutResolver); ok {
			return resolver
		}
	}

	resolver := &calloutResolver{}
	data, err := embeddedCalloutFiles.ReadFile("callouts/" + key + ".json")
	if err == nil {
		var payload calloutFile
		if jsonErr := json.Unmarshal(data, &payload); jsonErr == nil {
			resolver.areas = payload.Callouts
		}
	}

	calloutResolverCache.Store(key, resolver)
	return resolver
}

func normalizeCalloutMapKey(mapName string) string {
	mapName = strings.TrimSpace(strings.ToLower(mapName))
	if mapName == "" {
		return ""
	}

	mapName = strings.ReplaceAll(mapName, `\\`, "/")
	if slash := strings.LastIndex(mapName, "/"); slash >= 0 {
		mapName = mapName[slash+1:]
	}

	mapName = strings.TrimSuffix(mapName, ".json")
	mapName = strings.TrimPrefix(mapName, "de_")

	switch mapName {
	case "dust_2":
		return "dust2"
	default:
		return mapName
	}
}

func (r *calloutResolver) LabelForPlayer(player *common.Player) string {
	if player == nil {
		return ""
	}
	pos := player.Position()
	return r.LabelForPosition(float64(pos.X), float64(pos.Y), float64(pos.Z))
}

func (r *calloutResolver) LabelForPosition(x float64, y float64, z float64) string {
	if r == nil || len(r.areas) == 0 {
		return ""
	}
	if x == 0 && y == 0 && z == 0 {
		return ""
	}

	bestInsideName := ""
	bestInsideVolume := math.MaxFloat64
	bestInsideDistance := math.MaxFloat64
	bestNearbyName := ""
	bestNearbyDistance := math.MaxFloat64

	for _, area := range r.areas {
		name := strings.TrimSpace(area.Name)
		if name == "" {
			continue
		}

		minX, maxX := orderedBounds(area.MinX, area.MaxX)
		minY, maxY := orderedBounds(area.MinY, area.MaxY)
		minZ, maxZ := orderedBounds(area.MinZ, area.MaxZ)
		inside := x >= minX && x <= maxX && y >= minY && y <= maxY && z >= minZ-calloutVerticalTolerance && z <= maxZ+calloutVerticalTolerance
		distance := distanceToCalloutBounds(x, y, z, minX, maxX, minY, maxY, minZ, maxZ)
		originDistance := distanceBetweenPoints(x, y, z, area.OriginX, area.OriginY, area.OriginZ)

		if inside {
			volume := math.Max(maxX-minX, 1) * math.Max(maxY-minY, 1) * math.Max(maxZ-minZ, 1)
			if volume < bestInsideVolume || (volume == bestInsideVolume && originDistance < bestInsideDistance) {
				bestInsideName = name
				bestInsideVolume = volume
				bestInsideDistance = originDistance
			}
			continue
		}

		if distance < bestNearbyDistance {
			bestNearbyName = name
			bestNearbyDistance = distance
		}
	}

	if bestInsideName != "" {
		return bestInsideName
	}
	if bestNearbyDistance <= calloutSnapDistance {
		return bestNearbyName
	}
	return ""
}

func applyEventLocationLabels(event *entities.RoundEvent, eventLocation string, actorLocation string, targetLocation string) {
	if event == nil {
		return
	}
	event.LocationLabel = strings.TrimSpace(eventLocation)
	event.ActorLocationLabel = strings.TrimSpace(actorLocation)
	event.TargetLocationLabel = strings.TrimSpace(targetLocation)
}

func formatPlayerLabelWithLocation(name string, location string) string {
	label := strings.TrimSpace(name)
	if label == "" {
		return ""
	}
	location = strings.TrimSpace(location)
	if location == "" {
		return label
	}
	return label + " (" + location + ")"
}

func playerPositionCoordinates(player *common.Player) (float64, float64, float64) {
	if player == nil {
		return 0, 0, 0
	}
	pos := player.Position()
	return float64(pos.X), float64(pos.Y), float64(pos.Z)
}

func orderedBounds(a float64, b float64) (float64, float64) {
	if a <= b {
		return a, b
	}
	return b, a
}

func distanceToCalloutBounds(x float64, y float64, z float64, minX float64, maxX float64, minY float64, maxY float64, minZ float64, maxZ float64) float64 {
	dx := axisDistance(x, minX, maxX)
	dy := axisDistance(y, minY, maxY)
	dz := axisDistance(z, minZ-calloutVerticalTolerance, maxZ+calloutVerticalTolerance)
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func axisDistance(value float64, min float64, max float64) float64 {
	switch {
	case value < min:
		return min - value
	case value > max:
		return value - max
	default:
		return 0
	}
}

func distanceBetweenPoints(x1 float64, y1 float64, z1 float64, x2 float64, y2 float64, z2 float64) float64 {
	dx := x1 - x2
	dy := y1 - y2
	dz := z1 - z2
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}
