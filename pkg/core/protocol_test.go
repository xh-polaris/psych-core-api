package core

import "testing"

func TestDecodeInterruptInput(t *testing.T) {
	t.Run("explicit target id supports the first turn", func(t *testing.T) {
		input, err := DecodeInterruptInput(map[string]any{"targetId": float64(0)})
		if err != nil {
			t.Fatal(err)
		}
		if input.TargetID == nil || *input.TargetID != 0 {
			t.Fatalf("target id = %v, want 0", input.TargetID)
		}
	})

	t.Run("missing target id is accepted for legacy clients", func(t *testing.T) {
		input, err := DecodeInterruptInput(nil)
		if err != nil {
			t.Fatal(err)
		}
		if input.TargetID != nil {
			t.Fatalf("target id = %v, want nil", input.TargetID)
		}
	})

	t.Run("legacy string content targets the active turn", func(t *testing.T) {
		input, err := DecodeInterruptInput("interrupt")
		if err != nil {
			t.Fatal(err)
		}
		if input.TargetID != nil {
			t.Fatalf("target id = %v, want nil", input.TargetID)
		}
	})
}
