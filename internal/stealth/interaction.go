package stealth

import (
	"math"
	"math/rand/v2"
)

// Action is a high-level target the caller wants to perform.
type Action struct {
	Type string `json:"type"` // click | type
	Text string `json:"text,omitempty"`
	X    int    `json:"x,omitempty"`
	Y    int    `json:"y,omitempty"`
}

// InteractionParams is the input to PlanInteraction.
type InteractionParams struct {
	Actions []Action
	Seed    uint64
}

// Step is one low-level humanized event with a relative timestamp (ms).
type Step struct {
	Type string  `json:"type"` // move | down | up | key
	Key  string  `json:"key,omitempty"`
	AtMs float64 `json:"at_ms"`
	X    int     `json:"x,omitempty"`
	Y    int     `json:"y,omitempty"`
}

// InteractionPlan is a humanized timeline the caller replays.
type InteractionPlan struct {
	Steps []Step `json:"steps"`
}

// PlanInteraction converts target actions into a humanized event timeline:
// cubic-Bézier mouse paths with jitter and Fitts-law-derived durations, plus
// per-key typing cadence. Deterministic for a given Seed.
func PlanInteraction(p InteractionParams) InteractionPlan {
	r := rand.New(rand.NewPCG(p.Seed, p.Seed^0xa0761d6478bd642f)) //nolint:gosec // math/rand intentional: humanized timing is not a security primitive
	var steps []Step
	t := 0.0
	cx, cy := 0, 0
	for _, a := range p.Actions {
		switch a.Type {
		case "click":
			seg, dt := mousePath(r, cx, cy, a.X, a.Y, t)
			steps = append(steps, seg...)
			t += dt
			steps = append(steps, Step{Type: "down", X: a.X, Y: a.Y, AtMs: t})
			t += 40 + r.Float64()*60 // dwell
			steps = append(steps, Step{Type: "up", X: a.X, Y: a.Y, AtMs: t})
			t += 60 + r.Float64()*120
			cx, cy = a.X, a.Y
		case "type":
			for _, ch := range a.Text {
				t += 60 + r.Float64()*120 // inter-key cadence
				steps = append(steps, Step{Type: "key", Key: string(ch), AtMs: t})
			}
		}
	}
	return InteractionPlan{Steps: steps}
}

func mousePath(r *rand.Rand, x0, y0, x1, y1 int, start float64) ([]Step, float64) {
	dist := math.Hypot(float64(x1-x0), float64(y1-y0))
	// Fitts-law-ish duration; bounded.
	dur := 120 + 100*math.Log2(1+dist/40)
	n := 12 + r.IntN(8)
	// random control points around the straight line for a curved path
	c1x := float64(x0) + (float64(x1-x0))*0.3 + (r.Float64()-0.5)*dist*0.3
	c1y := float64(y0) + (float64(y1-y0))*0.3 + (r.Float64()-0.5)*dist*0.3
	c2x := float64(x0) + (float64(x1-x0))*0.7 + (r.Float64()-0.5)*dist*0.3
	c2y := float64(y0) + (float64(y1-y0))*0.7 + (r.Float64()-0.5)*dist*0.3
	steps := make([]Step, 0, n)
	for i := 1; i <= n; i++ {
		u := float64(i) / float64(n)
		bx := cubic(u, float64(x0), c1x, c2x, float64(x1))
		by := cubic(u, float64(y0), c1y, c2y, float64(y1))
		jitter := 0.0
		if i < n { // no jitter on the final landing point
			jitter = (r.Float64() - 0.5) * 2
		}
		steps = append(steps, Step{
			Type: "move",
			X:    int(bx + jitter),
			Y:    int(by + jitter),
			AtMs: start + dur*u,
		})
	}
	return steps, dur
}

func cubic(u, p0, p1, p2, p3 float64) float64 {
	v := 1 - u
	return v*v*v*p0 + 3*v*v*u*p1 + 3*v*u*u*p2 + u*u*u*p3
}
