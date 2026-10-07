package eval

// Expose unexported functions for white-box tests in the eval_test package.

var (
	ComputeCost        = computeCost
	ParseToolSelection = parseToolSelection
	ExtractJSON        = extractJSON
	ArgsMatchScore     = argsMatchScore
	BuildUserMessage   = buildUserMessage
	PRStubDesc         = prePRStubDesc
	IsFromService      = isFromService
)
