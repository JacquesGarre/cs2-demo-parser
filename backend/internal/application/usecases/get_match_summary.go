package usecases

import (
	"fmt"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
	"github.com/jcqsg/cs2-demos/backend/internal/domain/repositories"
)

type GetMatchSummaryUseCase struct {
	summaryRepository repositories.SummaryRepository
}

func NewGetMatchSummaryUseCase(summaryRepository repositories.SummaryRepository) GetMatchSummaryUseCase {
	return GetMatchSummaryUseCase{summaryRepository: summaryRepository}
}

func (uc GetMatchSummaryUseCase) Execute(demoID string) (entities.MatchSummary, error) {
	summary, found := uc.summaryRepository.GetByDemoID(demoID)
	if !found {
		return entities.MatchSummary{}, fmt.Errorf("summary not found")
	}

	return summary, nil
}
