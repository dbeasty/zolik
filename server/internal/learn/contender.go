package learn

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"zolik/server/internal/module"
)

// ParseContender reads a bench contender: a skill (easy, medium, hard), a
// style the game supplies, or net:<path>[@temperature] for a trained model,
// played by NetBot with the game's heuristic as fallback.
func ParseContender(g Benchable, spec string) (Contender, error) {
	for _, s := range module.Skills {
		if string(s) == spec {
			return Contender{Name: spec, Bot: g.Heuristic(), Skill: s}, nil
		}
	}
	if st, ok := g.(Styled); ok {
		if bot, ok := st.Styles()[spec]; ok {
			return Contender{Name: spec, Bot: bot}, nil
		}
	}
	if path, ok := strings.CutPrefix(spec, "net:"); ok {
		lg, ok := g.(Game)
		if !ok {
			return Contender{}, fmt.Errorf("%s cannot be played by a network yet", g.Name())
		}
		temp := 0.0
		if i := strings.LastIndex(path, "@"); i >= 0 {
			t, err := strconv.ParseFloat(path[i+1:], 64)
			if err != nil {
				return Contender{}, err
			}
			path, temp = path[:i], t
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return Contender{}, err
		}
		n, err := LoadNet(raw)
		if err != nil {
			return Contender{}, err
		}
		if n.StateDim != lg.StateDim() || n.CandDim != lg.CandDim() {
			return Contender{}, fmt.Errorf("%s was trained for another encoder", path)
		}
		return Contender{Name: spec, Bot: NetBot{Game: lg, Policy: NewPolicy(n), Fallback: g.Heuristic(), Temperature: temp}, Skill: module.SkillHard}, nil
	}
	return Contender{}, fmt.Errorf("unknown contender %q", spec)
}
