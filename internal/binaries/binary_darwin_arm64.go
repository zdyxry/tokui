//go:build darwin && arm64

package binaries

import _ "embed"

//go:embed embed/darwin_arm64/tokei.gz
var embeddedTokeiGz []byte
