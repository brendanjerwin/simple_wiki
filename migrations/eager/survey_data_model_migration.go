package eager

import (
	"fmt"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

const (
	surveysFMKey                 = "surveys"
	surveyMigratedDataModelFMKey = "migrated_data_model"
)

type SurveyDataModelMigrationScanJob struct {
	scanner       DataDirScanner
	coordinator   *jobs.JobQueueCoordinator
	readerMutator surveyPageModifier
}

func NewSurveyDataModelMigrationScanJob(
	scanner DataDirScanner,
	coordinator *jobs.JobQueueCoordinator,
	readerMutator surveyPageModifier,
) *SurveyDataModelMigrationScanJob {
	return &SurveyDataModelMigrationScanJob{
		scanner:       scanner,
		coordinator:   coordinator,
		readerMutator: readerMutator,
	}
}

func (*SurveyDataModelMigrationScanJob) GetName() string {
	return "SurveyDataModelMigrationScanJob"
}

func (j *SurveyDataModelMigrationScanJob) Execute() error {
	return enqueueDataModelMigrationJobs(
		j.scanner,
		j.coordinator,
		pageNeedsSurveyDataModelMigration,
		func(identifier string) jobs.Job {
			return NewSurveyDataModelMigrationJob(j.readerMutator, identifier)
		},
		"survey data-model",
	)
}

func pageNeedsSurveyDataModelMigration(fm map[string]any) bool {
	if wikipage.IsSystemPage(fm) {
		return false
	}
	return legacySurveysHaveAny(fm)
}

func legacySurveysHaveAny(fm map[string]any) bool {
	legacy, ok := fm[surveysFMKey].(map[string]any)
	if !ok {
		return false
	}
	for _, raw := range legacy {
		if _, ok := raw.(map[string]any); ok {
			return true
		}
	}
	return false
}

type SurveyDataModelMigrationJob struct {
	readerMutator surveyPageModifier
	identifier    string
}

type surveyPageModifier interface {
	wikipage.PageReaderMutator
	ModifyFrontMatterAndMarkdown(wikipage.PageIdentifier, func(wikipage.FrontMatter, wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error)) error
}

func NewSurveyDataModelMigrationJob(rw surveyPageModifier, id string) *SurveyDataModelMigrationJob {
	return &SurveyDataModelMigrationJob{readerMutator: rw, identifier: id}
}

func (j *SurveyDataModelMigrationJob) GetName() string {
	return fmt.Sprintf("SurveyDataModelMigrationJob-%s", j.identifier)
}

func (j *SurveyDataModelMigrationJob) Execute() error {
	id := wikipage.PageIdentifier(j.identifier)
	_, fm, err := j.readerMutator.ReadFrontMatter(id)
	if err != nil {
		return fmt.Errorf("read frontmatter for %s: %w", j.identifier, err)
	}
	if fm == nil {
		return nil
	}

	if !pageNeedsSurveyDataModelMigration(fm) {
		return nil
	}

	if err := j.readerMutator.ModifyFrontMatterAndMarkdown(id, func(currentFM wikipage.FrontMatter, md wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
		if currentFM == nil {
			return currentFM, md, nil
		}
		migrateSurveysIntoWikiNamespace(currentFM)
		return currentFM, md, nil
	}); err != nil {
		return fmt.Errorf("write migrated frontmatter for %s: %w", j.identifier, err)
	}
	return nil
}

func migrateSurveysIntoWikiNamespace(fm map[string]any) bool {
	legacySurveys, ok := fm[surveysFMKey].(map[string]any)
	if !ok || len(legacySurveys) == 0 {
		return false
	}

	wikiSurveys := ensureNestedMap(fm, wikiFMKey, surveysFMKey)
	changed := false
	for name, raw := range legacySurveys {
		legacySurvey, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, exists := wikiSurveys[name]; exists {
			changed = true
			continue
		}
		wikiSurvey := cloneSurveyMap(legacySurvey)
		wikiSurvey[surveyMigratedDataModelFMKey] = true
		wikiSurveys[name] = wikiSurvey
		changed = true
	}

	if changed {
		delete(fm, surveysFMKey)
	}
	return changed
}

func cloneSurveyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	for key, value := range in {
		out[key] = cloneSurveyValue(value)
	}
	return out
}

func cloneSurveyValue(value any) any {
	switch typed := value.(type) {
	case wikipage.FrontMatter:
		return cloneSurveyMap(map[string]any(typed))
	case map[string]any:
		return cloneSurveyMap(typed)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, cloneSurveyValue(item))
		}
		return out
	default:
		return typed
	}
}
