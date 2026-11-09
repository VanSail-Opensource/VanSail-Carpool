package httpapi

import (
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"testing"
)

func TestNodeWriteIgnoresObsoleteMultiplier(t *testing.T) {
	for _, input := range []string{`{}`, `{"multiplier":null}`, `{"multiplier":"1"}`, `{"multiplier":0}`, `{"multiplier":-1}`, `{"multiplier":1e99}`, `{"multiplier":[2]}`} {
		t.Run(input, func(t *testing.T) {
			var node xnode.Node
			if err := json.Unmarshal([]byte(input), &node); err != nil || node.Multiplier != 1 || node.MultiplierFP != xnode.MultiplierScale {
				t.Fatal("obsolete input affected node", node, err)
			}
		})
	}
}
