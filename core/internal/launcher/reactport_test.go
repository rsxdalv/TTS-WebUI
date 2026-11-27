package launcher

import "testing"

func TestReactPortFromEnv(t *testing.T) {
	for _, c := range []struct {
		env     string
		want    int
		wantErr bool
	}{
		{"", 0, false},
		{" 8120 ", 8120, false},
		{"0", 0, true},
		{"70000", 0, true},
		{"abc", 0, true},
	} {
		t.Setenv("REACT_UI_PORT", c.env)
		got, err := reactPortFromEnv()
		if got != c.want || (err != nil) != c.wantErr {
			t.Errorf("REACT_UI_PORT=%q: got %d, %v; want %d, error %v", c.env, got, err, c.want, c.wantErr)
		}
	}
}
