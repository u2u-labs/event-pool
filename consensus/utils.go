package consensus

import (
	"event-pool/types"
)

// BuildBlockParams are parameters passed into the BuildBlock helper method
type BuildBlockParams struct {
	Header *types.Header
}

// BuildBlock is a utility function that builds a block, based on the passed in header, filter and logs
func BuildBlock(params BuildBlockParams) *types.Block {
	header := params.Header

	header.ComputeHash()

	return &types.Block{
		Header: header,
	}
}
