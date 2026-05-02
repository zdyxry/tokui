//go:build linux && amd64

package binaries

import _ "embed"

//go:embed embed/linux_amd64/tokei.gz
var embeddedTokeiGz []byte
