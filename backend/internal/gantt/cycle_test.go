package gantt

import "testing"

func TestSelfLoop(t *testing.T) {
	ok, path := DetectCycle(nil, 7, 7)
	if !ok {
		t.Fatal("self-loop must be a cycle")
	}
	if len(path) < 2 || path[0] != 7 || path[len(path)-1] != 7 {
		t.Fatalf("path %v", path)
	}
}

func TestThreeNodeCycle(t *testing.T) {
	edges := []Edge{{1, 2}, {2, 3}}
	ok, path := DetectCycle(edges, 3, 1)
	if !ok {
		t.Fatal("A→B→C→A must be detected")
	}
	if len(path) < 4 {
		t.Fatalf("cycle path too short: %v", path)
	}
	if path[0] != path[len(path)-1] {
		t.Fatalf("path should close: %v", path)
	}
	seen := map[int64]bool{}
	for _, n := range path[:len(path)-1] {
		seen[n] = true
	}
	for _, n := range []int64{1, 2, 3} {
		if !seen[n] {
			t.Fatalf("node %d missing from %v", n, path)
		}
	}
}

func TestAcyclicAdd(t *testing.T) {
	edges := []Edge{{1, 2}, {2, 3}}
	if ok, path := DetectCycle(edges, 1, 3); ok {
		t.Fatalf("1→3 is a shortcut, not a cycle: %v", path)
	}
	if ok, _ := DetectCycle(edges, 4, 1); ok {
		t.Fatal("new source should be acyclic")
	}
}

func TestTwoNodeCycle(t *testing.T) {
	ok, path := DetectCycle([]Edge{{1, 2}}, 2, 1)
	if !ok {
		t.Fatal("1↔2 must cycle")
	}
	if len(path) < 3 {
		t.Fatalf("path %v", path)
	}
}

func TestEmptyGraph(t *testing.T) {
	if ok, _ := DetectCycle(nil, 1, 2); ok {
		t.Fatal("single edge is not a cycle")
	}
}

func TestDisconnectedCycle(t *testing.T) {
	edges := []Edge{{10, 11}, {11, 10}, {1, 2}}
	if ok, path := DetectCycle(edges, 2, 3); ok {
		t.Fatalf("unrelated existing cycle must not reject 2→3: %v", path)
	}
	if ok, _ := DetectCycle(edges, 11, 10); !ok {
		t.Fatal("re-adding 11→10 (already cyclic pair) is a cycle")
	}
}

func TestFourNodeNoCycle(t *testing.T) {
	edges := []Edge{{1, 2}, {1, 3}, {2, 4}, {3, 4}}
	if ok, path := DetectCycle(edges, 4, 5); ok {
		t.Fatalf("diamond plus sink: %v", path)
	}
}
