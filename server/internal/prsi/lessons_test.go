package prsi

import (
	"testing"

	"zolik/server/internal/lessontest"
)

func TestLessonsWalk(t *testing.T) { lessontest.Check(t, New()) }
