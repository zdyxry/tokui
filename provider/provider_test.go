package provider_test

import (
	"testing"

	"github.com/zdyxry/tokui/provider"
	"github.com/zdyxry/tokui/provider/scc"
	"github.com/zdyxry/tokui/tokei"
)

func TestCapabilityConstants(t *testing.T) {
	if provider.CapLines != 1 {
		t.Errorf("CapLines = %d, want 1", provider.CapLines)
	}
	if provider.CapComplexity != 2 {
		t.Errorf("CapComplexity = %d, want 2", provider.CapComplexity)
	}
	if provider.CapULOC != 4 {
		t.Errorf("CapULOC = %d, want 8", provider.CapULOC)
	}
}

func TestInfoCapabilities(t *testing.T) {
	tokeiInfo := tokei.New().Info()
	if tokeiInfo.Capabilities != provider.CapLines {
		t.Errorf("tokei capabilities = %v, want %v", tokeiInfo.Capabilities, provider.CapLines)
	}

	sccInfo := scc.New().Info()
	want := provider.CapLines | provider.CapComplexity
	if sccInfo.Capabilities != want {
		t.Errorf("scc capabilities = %v, want %v", sccInfo.Capabilities, want)
	}
}
