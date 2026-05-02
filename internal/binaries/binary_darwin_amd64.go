//go:build darwin && amd64

package binaries

import _ "embed"

//go:embed embed/darwin_amd64/tokei.gz
var embeddedTokeiGz []byte
