package routing

import _ "embed"

//go:embed routing_lexicon_default.json
var embeddedRoutingLexiconJSON []byte

//go:embed routing_weights_default.json
var embeddedRoutingWeightsJSON []byte
