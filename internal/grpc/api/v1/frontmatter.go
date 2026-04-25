package v1

import (
	"context"
	"os"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// reservedAgentKey is the top-level frontmatter key that AgentMetadataService
// owns exclusively. Generic Frontmatter writes touching this key are rejected
// to prevent accidental clobbering of cron schedules and chat memory.
const reservedAgentKey = "agent"

// reservedAgentRedirect is the error message users see when they try to mutate
// agent.* via the generic API. It points them at the right service.
const reservedAgentRedirect = "the 'agent' top-level frontmatter namespace is reserved; use AgentMetadataService instead"

// containsReservedAgentKey returns true when fm contains the reserved 'agent'
// top-level key. Used by Merge/Replace to reject writes that would touch it.
func containsReservedAgentKey(fm map[string]any) bool {
	if fm == nil {
		return false
	}
	_, ok := fm[reservedAgentKey]
	return ok
}

// pathStartsWithReservedAgent returns true when the first PathComponent is the
// reserved 'agent' key. Used by RemoveKeyAtPath to reject removal targeting
// agent or any agent.* descendant.
func pathStartsWithReservedAgent(path []*apiv1.PathComponent) bool {
	if len(path) == 0 {
		return false
	}
	keyComp, ok := path[0].Component.(*apiv1.PathComponent_Key)
	if !ok {
		return false
	}
	return keyComp.Key == reservedAgentKey
}

// preserveAgentSubtree carries the reserved agent.* subtree from existing into
// incoming so a ReplaceFrontmatter call that omits agent does not silently
// destroy it. Callers must already have rejected payloads that include agent.
func preserveAgentSubtree(existing, incoming map[string]any) {
	if existing == nil || incoming == nil {
		return
	}
	if agent, ok := existing[reservedAgentKey]; ok {
		incoming[reservedAgentKey] = agent
	}
}

// filterIdentifierKey removes the identifier key from a frontmatter map.
// It also removes the reserved agent.* namespace because that subtree is
// owned exclusively by AgentMetadataService — letting it through would let a
// generic frontmatter editor read it, send it back unchanged on save, and
// trip the agent.* write rejection. Hiding it from the read makes the edit
// dialog work without special-casing.
func filterIdentifierKey(fm map[string]any) map[string]any {
	if fm == nil {
		return nil
	}

	filtered := make(map[string]any)
	for k, v := range fm {
		if k == identifierKey || k == reservedAgentKey {
			continue
		}
		filtered[k] = v
	}
	return filtered
}

// validateNoIdentifierKey checks if the frontmatter contains an identifier key.
func validateNoIdentifierKey(fm map[string]any) error {
	if fm == nil {
		return nil
	}

	if _, exists := fm[identifierKey]; exists {
		return status.Error(codes.InvalidArgument, "identifier key cannot be modified")
	}
	return nil
}

func mergeFrontmatterDeep(target, source map[string]any) {
	for key, value := range source {
		nestedSource, ok := value.(map[string]any)
		if !ok {
			target[key] = value
			continue
		}

		existingValue := target[key]
		nestedTarget, ok := existingValue.(map[string]any)
		if !ok {
			nestedTarget = make(map[string]any)
			target[key] = nestedTarget
		}

		mergeFrontmatterDeep(nestedTarget, nestedSource)
	}
}

// isIdentifierKeyPath checks if the given path targets the identifier key at the root level.
func isIdentifierKeyPath(path []*apiv1.PathComponent) bool {
	if len(path) != 1 {
		return false
	}

	keyComp, ok := path[0].Component.(*apiv1.PathComponent_Key)
	if !ok {
		return false
	}

	return keyComp.Key == identifierKey
}

// removeAtPath recursively traverses the data structure according to the path
// and removes the element at the end of the path. It returns the modified data
// structure. For slices, this may be a new slice instance.
func removeAtPath(data any, path []*apiv1.PathComponent) (any, error) {
	if len(path) == 0 {
		// This should be caught by the public-facing method, but as a safeguard:
		return nil, status.Error(codes.InvalidArgument, "path cannot be empty")
	}

	component := path[0]
	remainingPath := path[1:]

	switch v := data.(type) {
	case map[string]any:
		return removeAtPathFromMap(v, component, remainingPath)
	case []any:
		return removeAtPathFromSlice(v, component, remainingPath)
	default:
		// Trying to traverse deeper, but `data` is a primitive.
		return nil, status.Error(codes.InvalidArgument, "path is deeper than data structure")
	}
}

// removeAtPathFromMap handles the map case for removeAtPath.
func removeAtPathFromMap(v map[string]any, component *apiv1.PathComponent, remainingPath []*apiv1.PathComponent) (any, error) {
	keyComp, ok := component.Component.(*apiv1.PathComponent_Key)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "path component is not a key for a map: %T", component.Component)
	}
	key := keyComp.Key

	value, exists := v[key]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "key '%s' not found", key)
	}

	if len(remainingPath) == 0 {
		// Base case: remove key from map
		delete(v, key)
		return v, nil
	}

	// Recursive step
	newValue, err := removeAtPath(value, remainingPath)
	if err != nil {
		return nil, err
	}
	v[key] = newValue
	return v, nil
}

// removeAtPathFromSlice handles the slice case for removeAtPath.
func removeAtPathFromSlice(v []any, component *apiv1.PathComponent, remainingPath []*apiv1.PathComponent) (any, error) {
	indexComp, ok := component.Component.(*apiv1.PathComponent_Index)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "path component is not an index for a slice: %T", component.Component)
	}
	idx := int(indexComp.Index)

	if idx < 0 || idx >= len(v) {
		return nil, status.Errorf(codes.OutOfRange, "index %d is out of range for slice of length %d", idx, len(v))
	}

	if len(remainingPath) == 0 {
		// Base case: remove item from slice, zeroing the vacated slot to prevent memory leaks.
		copy(v[idx:], v[idx+1:])
		v[len(v)-1] = nil
		return v[:len(v)-1], nil
	}

	// Recursive step
	newValue, err := removeAtPath(v[idx], remainingPath)
	if err != nil {
		return nil, err
	}
	v[idx] = newValue
	return v, nil
}

// GetFrontmatter implements the GetFrontmatter RPC.
func (s *Server) GetFrontmatter(_ context.Context, req *apiv1.GetFrontmatterRequest) (resp *apiv1.GetFrontmatterResponse, err error) {
	var fm map[string]any
	_, fm, err = s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(req.Page))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, pageNotFoundErrFmt, req.Page)
		}
		return nil, status.Errorf(codes.Internal, failedToReadFrontmatterErrFmt, err)
	}

	// Filter out the identifier key from the response
	filteredFm := filterIdentifierKey(fm)

	var structFm *structpb.Struct
	structFm, err = structpb.NewStruct(filteredFm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert frontmatter to struct: %v", err)
	}

	return &apiv1.GetFrontmatterResponse{
		Frontmatter: structFm,
	}, nil
}

// MergeFrontmatter implements the MergeFrontmatter RPC.
func (s *Server) MergeFrontmatter(_ context.Context, req *apiv1.MergeFrontmatterRequest) (resp *apiv1.MergeFrontmatterResponse, err error) {
	// Validate that the request doesn't contain an identifier key
	if req.Frontmatter != nil {
		newFm := req.Frontmatter.AsMap()
		if err := validateNoIdentifierKey(newFm); err != nil {
			return nil, err
		}
		if containsReservedAgentKey(newFm) {
			return nil, status.Error(codes.InvalidArgument, reservedAgentRedirect)
		}
	}

	_, existingFm, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(req.Page))
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, failedToReadFrontmatterErrFmt, err)
	}

	if existingFm == nil {
		existingFm = make(map[string]any)
	}

	if req.Frontmatter != nil {
		newFm := req.Frontmatter.AsMap()
		mergeFrontmatterDeep(existingFm, newFm)
	}

	err = s.pageReaderMutator.WriteFrontMatter(wikipage.PageIdentifier(req.Page), existingFm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, failedToWriteFrontmatterErrFmt, err)
	}

	// Filter out the identifier key from the response
	filteredFm := filterIdentifierKey(existingFm)
	mergedFmStruct, err := structpb.NewStruct(filteredFm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert merged frontmatter to struct: %v", err)
	}

	return &apiv1.MergeFrontmatterResponse{
		Frontmatter: mergedFmStruct,
	}, nil
}

// ReplaceFrontmatter implements the ReplaceFrontmatter RPC.
func (s *Server) ReplaceFrontmatter(_ context.Context, req *apiv1.ReplaceFrontmatterRequest) (resp *apiv1.ReplaceFrontmatterResponse, err error) {
	var fm map[string]any
	if req.Frontmatter != nil {
		fm = req.Frontmatter.AsMap()
		if containsReservedAgentKey(fm) {
			return nil, status.Error(codes.InvalidArgument, reservedAgentRedirect)
		}
		// Filter out any user-provided identifier key and set the correct one
		fm = filterIdentifierKey(fm)
		fm[identifierKey] = req.Page
	}

	// Carry the reserved agent.* subtree forward — even a caller unaware of the
	// namespace must not be able to destroy it via Replace.
	if fm != nil {
		_, existingFm, readErr := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(req.Page))
		if readErr != nil && !os.IsNotExist(readErr) {
			return nil, status.Errorf(codes.Internal, failedToReadFrontmatterErrFmt, readErr)
		}
		preserveAgentSubtree(existingFm, fm)
	}

	err = s.pageReaderMutator.WriteFrontMatter(wikipage.PageIdentifier(req.Page), fm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, failedToWriteFrontmatterErrFmt, err)
	}

	// Return the frontmatter without the identifier key
	var responseFm map[string]any
	if fm != nil {
		responseFm = filterIdentifierKey(fm)
	}

	var responseFmStruct *structpb.Struct
	if len(responseFm) > 0 {
		responseFmStruct, err = structpb.NewStruct(responseFm)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to convert frontmatter to struct: %v", err)
		}
	}

	return &apiv1.ReplaceFrontmatterResponse{
		Frontmatter: responseFmStruct,
	}, nil
}

// RemoveKeyAtPath implements the RemoveKeyAtPath RPC.
func (s *Server) RemoveKeyAtPath(_ context.Context, req *apiv1.RemoveKeyAtPathRequest) (*apiv1.RemoveKeyAtPathResponse, error) {
	if len(req.GetKeyPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "key_path cannot be empty")
	}

	// Validate that the path is not targeting the identifier key
	if isIdentifierKeyPath(req.GetKeyPath()) {
		return nil, status.Error(codes.InvalidArgument, "identifier key cannot be removed")
	}

	// Reject any path under the reserved agent.* namespace.
	if pathStartsWithReservedAgent(req.GetKeyPath()) {
		return nil, status.Error(codes.InvalidArgument, reservedAgentRedirect)
	}

	_, fm, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(req.Page))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, pageNotFoundErrFmt, req.Page)
		}
		return nil, status.Errorf(codes.Internal, failedToReadFrontmatterErrFmt, err)
	}

	if fm == nil {
		// Attempting to remove from a non-existent frontmatter. The path will not be found.
		fm = make(map[string]any)
	}

	updatedFm, err := removeAtPath(map[string]any(fm), req.GetKeyPath())
	if err != nil {
		return nil, err
	}

	err = s.pageReaderMutator.WriteFrontMatter(wikipage.PageIdentifier(req.Page), wikipage.FrontMatter(updatedFm.(map[string]any)))
	if err != nil {
		return nil, status.Errorf(codes.Internal, failedToWriteFrontmatterErrFmt, err)
	}

	// Filter out the identifier key from the response
	filteredFm := filterIdentifierKey(updatedFm.(map[string]any))
	updatedFmStruct, err := structpb.NewStruct(filteredFm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert updated frontmatter to struct: %v", err)
	}

	return &apiv1.RemoveKeyAtPathResponse{
		Frontmatter: updatedFmStruct,
	}, nil
}
