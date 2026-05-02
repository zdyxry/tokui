//go:build linux && arm64

package binaries

import _ "embed"

//go:embed embed/linux_arm64/tokei.gz
var embeddedTokeiGz []byte
