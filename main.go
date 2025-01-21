package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"time"

	"github.com/notnil/chess"
	"github.com/notnil/chess/uci"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("disagree: ")

	if len(os.Args) != 3 {
		log.Fatalf("usage: disagree PATH1 PATH2")
	}

	e1, err := NewEngine(os.Args[1])
	if err != nil {
		log.Fatalf("error: engine 1: create: %v", err)
	}
	defer e1.Close()

	e2, err := NewEngine(os.Args[2])
	if err != nil {
		log.Fatalf("error: engine 2: create: %v", err)
	}
	defer e2.Close()

	if err := e1.Ready(); err != nil {
		log.Fatalf("error: engine 1: ready: %v", err)
	}

	if err := e2.Ready(); err != nil {
		log.Fatalf("error: engine 2: ready: %v", err)
	}

	var (
		worstPerDiff  float64
		worstEval1    int
		worstEval2    int
		worstPosition string
	)

	for {
		game := chess.NewGame()
		for game.Outcome() == chess.NoOutcome {
			eval1, err := e1.Eval(game)
			if err != nil {
				log.Fatalf("engine 1: %v", err)
			}

			eval2, err := e2.Eval(game)
			if err != nil {
				log.Fatalf("engine 2: %v", err)
			}

			pd := perDiff(eval1, eval2)

			// TODO(clfs): Remove temporary non-zero check.
			if eval1 != 0 && eval2 != 0 && pd > worstPerDiff {
				worstPerDiff = pd
				worstEval1 = eval1
				worstEval2 = eval2
				worstPosition = game.Position().String()
				fmt.Printf("%d != %d\t%s\n", worstEval1, worstEval2, worstPosition)
			}

			moves := game.ValidMoves()
			move := moves[rand.IntN(len(moves))]
			game.Move(move)
		}
	}
}

func perDiff(x, y int) float64 {
	if x == 0 && y == 0 {
		return 0
	}
	return float64(abs(x-y)) / float64(max(x, y))
}

type Disagreement struct {
	Eval1    int
	Eval2    int
	Position string
}

func (d Disagreement) Less(other Disagreement) bool {
	return abs(d.Eval1-d.Eval2) < abs(other.Eval1-other.Eval2)
}

func Search(e1, e2 *Engine, g *chess.Game, best *Disagreement) (*Disagreement, error) {
	log.Printf("pos=%v best=%v", g.Position().String(), best)

	if outcome := g.Outcome(); outcome != chess.NoOutcome {
		return best, nil
	}

	eval1, err := e1.Eval(g)
	if err != nil {
		return nil, fmt.Errorf("engine 1: %v", err)
	}

	eval2, err := e2.Eval(g)
	if err != nil {
		return nil, fmt.Errorf("engine 2: %v", err)
	}

	d := &Disagreement{
		Eval1:    eval1,
		Eval2:    eval2,
		Position: g.Position().String(),
	}

	if d.Less(*best) {
		return best, nil
	}

	moves := g.ValidMoves()

	rand.Shuffle(len(moves), func(i, j int) {
		mi := *moves[i]
		mj := *moves[j]
		moves[j] = &mi
		moves[i] = &mj
	})

	for _, m := range moves {
		g := g.Clone()
		if err := g.Move(m); err != nil {
			return nil, fmt.Errorf("move: %v", err)
		}

		d2, err := Search(e1, e2, g, best)
		if err != nil {
			return nil, fmt.Errorf("search: %v", err)
		}

		if d.Less(*d2) {
			d = d2
			log.Printf("best: %s", d)
		}
	}

	return d, nil
}

type Engine struct {
	e *uci.Engine
}

func NewEngine(path string) (*Engine, error) {
	e, err := uci.New(path)
	if err != nil {
		return nil, fmt.Errorf("new engine: %v", err)
	}
	return &Engine{e: e}, nil
}

func (e *Engine) Ready() error {
	if err := e.e.Run(uci.CmdUCI, uci.CmdIsReady); err != nil {
		return fmt.Errorf("ready: %v", err)
	}
	return nil
}

func (e *Engine) Eval(g *chess.Game) (int, error) {
	cmdPos := uci.CmdPosition{Position: g.Position()}
	cmdGo := uci.CmdGo{MoveTime: 50 * time.Millisecond}
	if err := e.e.Run(cmdPos, cmdGo); err != nil {
		return 0, fmt.Errorf("eval: %v", err)
	}
	return e.e.SearchResults().Info.Score.CP, nil
}

func (e *Engine) Close() error {
	if err := e.e.Close(); err != nil {
		return fmt.Errorf("close: %v", err)
	}
	return nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
