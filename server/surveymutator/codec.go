package surveymutator

import (
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

const (
	surveysKey     = "surveys"
	questionKey    = "question"
	fieldsKey      = "fields"
	responsesKey   = "responses"
	closedKey      = "closed"
	updatedAtKey   = "updated_at"
	nameKey        = "name"
	typeKey        = "type"
	labelKey       = "label"
	requiredKey    = "required"
	optionsKey     = "options"
	minKey         = "min"
	maxKey         = "max"
	userKey        = "user"
	anonymousKey   = "anonymous"
	submittedAtKey = "submitted_at"
	valuesKey      = "values"
)

func surveyExists(fm wikipage.FrontMatter, name string) bool {
	return readMap(readMap(fm, surveysKey), name) != nil
}

func decodeSurvey(fm wikipage.FrontMatter, name string) *apiv1.Survey {
	raw := readMap(readMap(fm, surveysKey), name)
	out := &apiv1.Survey{
		Name:     name,
		Question: stringValue(raw, questionKey),
		Closed:   boolValue(raw, closedKey),
	}
	if t, ok := readTimestamp(raw, updatedAtKey); ok {
		out.UpdatedAt = t
	}
	for _, rawField := range readSlice(raw, fieldsKey) {
		fieldMap, ok := rawField.(map[string]any)
		if !ok {
			continue
		}
		out.Fields = append(out.Fields, decodeField(fieldMap))
	}
	for _, rawResponse := range readSlice(raw, responsesKey) {
		responseMap, ok := rawResponse.(map[string]any)
		if !ok {
			continue
		}
		out.Responses = append(out.Responses, decodeResponse(responseMap))
	}
	sortResponses(out.Responses)
	return out
}

func encodeSurvey(fm wikipage.FrontMatter, name string, survey *apiv1.Survey) {
	surveys := ensureMap(fm, surveysKey)
	out := map[string]any{
		questionKey:  survey.GetQuestion(),
		fieldsKey:    encodeFields(survey.GetFields()),
		responsesKey: encodeResponses(survey.GetResponses()),
		closedKey:    survey.GetClosed(),
	}
	if survey.GetUpdatedAt() != nil {
		out[updatedAtKey] = survey.GetUpdatedAt().AsTime().Format(time.RFC3339Nano)
	}
	surveys[name] = out
}

func decodeField(raw map[string]any) *apiv1.SurveyField {
	field := &apiv1.SurveyField{
		Name:    stringValue(raw, nameKey),
		Type:    stringValue(raw, typeKey),
		Options: stringSlice(raw, optionsKey),
	}
	if v := stringValue(raw, labelKey); v != "" {
		field.Label = &v
	}
	if v, ok := raw[requiredKey].(bool); ok {
		field.Required = &v
	}
	if v, ok := float64Value(raw, minKey); ok {
		field.Min = &v
	}
	if v, ok := float64Value(raw, maxKey); ok {
		field.Max = &v
	}
	return field
}

func encodeFields(fields []*apiv1.SurveyField) []any {
	out := make([]any, 0, len(fields))
	for _, field := range fields {
		if field == nil {
			continue
		}
		raw := map[string]any{
			nameKey: field.GetName(),
			typeKey: field.GetType(),
		}
		if field.Label != nil {
			raw[labelKey] = field.GetLabel()
		}
		if field.Required != nil {
			raw[requiredKey] = field.GetRequired()
		}
		if len(field.GetOptions()) > 0 {
			raw[optionsKey] = append([]string(nil), field.GetOptions()...)
		}
		if field.Min != nil {
			raw[minKey] = field.GetMin()
		}
		if field.Max != nil {
			raw[maxKey] = field.GetMax()
		}
		out = append(out, raw)
	}
	return out
}

func decodeResponse(raw map[string]any) *apiv1.SurveyResponse {
	response := &apiv1.SurveyResponse{
		User:      stringValue(raw, userKey),
		Anonymous: boolValue(raw, anonymousKey),
		Values:    structFromMap(readMap(raw, valuesKey)),
	}
	if t, ok := readTimestamp(raw, submittedAtKey); ok {
		response.SubmittedAt = t
	}
	return response
}

func encodeResponses(responses []*apiv1.SurveyResponse) []any {
	out := make([]any, 0, len(responses))
	for _, response := range responses {
		if response == nil {
			continue
		}
		raw := map[string]any{
			userKey:      response.GetUser(),
			anonymousKey: response.GetAnonymous(),
			valuesKey:    structToMap(response.GetValues()),
		}
		if response.GetSubmittedAt() != nil {
			raw[submittedAtKey] = response.GetSubmittedAt().AsTime().Format(time.RFC3339Nano)
		}
		out = append(out, raw)
	}
	return out
}

func readMap(raw map[string]any, key string) map[string]any {
	if raw == nil {
		return nil
	}
	switch v := raw[key].(type) {
	case map[string]any:
		return v
	case wikipage.FrontMatter:
		return map[string]any(v)
	default:
		return nil
	}
}

func ensureMap(raw map[string]any, key string) map[string]any {
	if existing := readMap(raw, key); existing != nil {
		return existing
	}
	next := map[string]any{}
	raw[key] = next
	return next
}

func readSlice(raw map[string]any, key string) []any {
	if raw == nil {
		return nil
	}
	switch v := raw[key].(type) {
	case []any:
		return v
	case []map[string]any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}

func stringValue(raw map[string]any, key string) string {
	if raw == nil {
		return ""
	}
	v, _ := raw[key].(string)
	return v
}

func boolValue(raw map[string]any, key string) bool {
	if raw == nil {
		return false
	}
	v, _ := raw[key].(bool)
	return v
}

func stringSlice(raw map[string]any, key string) []string {
	if raw == nil {
		return nil
	}
	if values, ok := raw[key].([]string); ok {
		return append([]string(nil), values...)
	}
	values := readSlice(raw, key)
	out := make([]string, 0, len(values))
	for _, rawValue := range values {
		value, ok := rawValue.(string)
		if ok {
			out = append(out, value)
		}
	}
	return out
}

func float64Value(raw map[string]any, key string) (float64, bool) {
	if raw == nil {
		return 0, false
	}
	switch v := raw[key].(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

func readTimestamp(raw map[string]any, key string) (*timestamppb.Timestamp, bool) {
	value := stringValue(raw, key)
	if value == "" {
		return nil, false
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, false
	}
	return timestamppb.New(t), true
}

func structFromMap(values map[string]any) *structpb.Struct {
	if values == nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	out, err := structpb.NewStruct(values)
	if err != nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	return out
}

func structToMap(values *structpb.Struct) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	return values.AsMap()
}
