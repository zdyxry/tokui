//go:build windows && amd64

package binaries

import _ "embed"

//go:embed embed/windows_amd64/tokei.gz
var embeddedTokeiGz []byte
