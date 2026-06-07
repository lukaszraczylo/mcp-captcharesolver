package stealth

import "testing"

func TestPlanInteraction(t *testing.T) {
	plan := PlanInteraction(InteractionParams{
		Actions: []Action{{Type: "click", X: 50, Y: 50}, {Type: "click", X: 150, Y: 150}},
		Seed:    7,
	})
	if len(plan.Steps) == 0 {
		t.Fatal("no steps")
	}
	// monotonic non-decreasing timing
	last := -1.0
	for _, s := range plan.Steps {
		if s.AtMs < last {
			t.Fatalf("timing not monotonic: %v then %v", last, s.AtMs)
		}
		last = s.AtMs
	}
	// last mouse step must land on the final target
	var lastMove *Step
	for i := range plan.Steps {
		if plan.Steps[i].Type == "move" {
			lastMove = &plan.Steps[i]
		}
	}
	if lastMove == nil || lastMove.X != 150 || lastMove.Y != 150 {
		t.Fatalf("final move did not reach target: %+v", lastMove)
	}
}
