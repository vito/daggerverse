package knowledge

import _ "embed"

//go:embed sdk/go.md
var GoSDK string

// TODO: since it's dagger_query -> learn_querying, will it still run the
// original bogus query, or will it immediately apply learnings from the schema
//
//go:embed querying.md
var Querying string
