package tracker

import (
	"encoding/json"
	"testing"
)

func TestSourceIDsJSONUsesNullForMissingProvider(t *testing.T) {
	ids := SourceIDsFromURLs([]string{"https://www.nexusmods.com/7daystodie/mods/870"})
	data, err := json.Marshal(ids)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"nexus":"870","7d2dmods":null}`
	if string(data) != want {
		t.Fatalf("got %s, want %s", data, want)
	}
}

func TestSourceIDsRoundTrip(t *testing.T) {
	ids := SourceIDsFromURLs([]string{
		"https://www.nexusmods.com/7daystodie/mods/870",
		"https://7daystodiemods.com/mods/agf-v3-hudplus-1main/",
	})
	var decoded SourceIDs
	data, _ := json.Marshal(ids)
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	urls := decoded.URLs()
	if len(urls) != 2 {
		t.Fatalf("unexpected URLs: %#v", urls)
	}
}

func TestSourceIDsFromShortInputs(t *testing.T) {
	ids, err := SourceIDsFromInputs("870", "smart-doors")
	if err != nil {
		t.Fatal(err)
	}
	if ids.NexusValue() != "870" || ids.SevenD2DValue() != "smart-doors" {
		t.Fatalf("unexpected IDs: %#v", ids)
	}
}

func TestSourceIDsFromFullURLInputs(t *testing.T) {
	ids, err := SourceIDsFromInputs(
		"https://www.nexusmods.com/7daystodie/mods/870",
		"https://7daystodiemods.com/mods/smart-doors/",
	)
	if err != nil {
		t.Fatal(err)
	}
	if ids.NexusValue() != "870" || ids.SevenD2DValue() != "smart-doors" {
		t.Fatalf("unexpected IDs: %#v", ids)
	}
}
