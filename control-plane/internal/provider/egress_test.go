package provider

import "testing"

// The ladder the platform ships with, least sensitive first.
var ladder = []string{"public", "internal", "confidential", "restricted"}

func TestRouteRefusesContentAboveTheProviderCeiling(t *testing.T) {
	// A cloud provider added for development is public-only by default.
	route := Route{Logical: "default", Provider: "openai", MaxClassification: "public"}

	if !route.Permits(ladder, "public") {
		t.Fatal("a public-only provider refused public content")
	}
	for _, level := range []string{"internal", "confidential", "restricted"} {
		if route.Permits(ladder, level) {
			t.Fatalf("a public-only provider accepted %s content", level)
		}
	}
}

func TestRouteAllowsUpToItsCeiling(t *testing.T) {
	route := Route{Provider: "openai", MaxClassification: "confidential"}

	for _, level := range []string{"public", "internal", "confidential"} {
		if !route.Permits(ladder, level) {
			t.Fatalf("a confidential provider refused %s content", level)
		}
	}
	if route.Permits(ladder, "restricted") {
		t.Fatal("a confidential provider accepted restricted content")
	}
}

func TestRouteWithNoCeilingIsUnrestricted(t *testing.T) {
	// Routes parsed from the legacy MODEL_ROUTES variable carry no ceiling and
	// only ever pointed at a local provider.
	route := Route{Provider: "ollama"}

	for _, level := range ladder {
		if !route.Permits(ladder, level) {
			t.Fatalf("an unrestricted route refused %s content", level)
		}
	}
}

func TestRouteRefusesAClassificationTheLadderDoesNotContain(t *testing.T) {
	route := Route{Provider: "openai", MaxClassification: "public"}

	// A level nobody has heard of means the corpus and the routing table disagree
	// about what levels exist. Guessing in that state is how content escapes.
	if route.Permits(ladder, "secret") {
		t.Fatal("an unknown classification was allowed through")
	}
	// The same in the other direction: a ceiling that is not on the ladder is a
	// misconfigured provider, and a misconfigured provider must not be trusted.
	unknown := Route{Provider: "openai", MaxClassification: "top-secret"}
	if unknown.Permits(ladder, "public") {
		t.Fatal("a provider with an unknown ceiling was trusted")
	}
}
