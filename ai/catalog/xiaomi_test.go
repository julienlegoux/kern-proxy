package catalog

// Ports: packages/ai/test/xiaomi-models.test.ts (via getBuiltinModel/
// getBuiltinModels, upstream's compat.ts getModel/getModels aliases —
// compat.ts itself isn't ported, see docs/PORTING.md's "Intentional
// deviations").

import "testing"

func TestXiaomiMiMoModels_KeepsFlashOnAPIBillingProvider(t *testing.T) {
	if BuiltinModel("xiaomi", "mimo-v2-flash") == nil {
		t.Fatal(`BuiltinModel("xiaomi", "mimo-v2-flash") = nil, want a model`)
	}
}

func TestXiaomiMiMoModels_OmitsFlashFromTokenPlanProviders(t *testing.T) {
	for _, provider := range []string{"xiaomi-token-plan-cn", "xiaomi-token-plan-ams", "xiaomi-token-plan-sgp"} {
		t.Run(provider, func(t *testing.T) {
			for _, m := range BuiltinModels(provider) {
				if m.ID == "mimo-v2-flash" {
					t.Fatalf("%s unexpectedly includes mimo-v2-flash", provider)
				}
			}
		})
	}
}
