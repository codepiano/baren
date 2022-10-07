package utils

import "testing"

func TestCleanLabelText(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "single tail",
			args: args{"abc #abc"},
			want: "abc",
		},
		{
			name: "single middle",
			args: args{"abc #abc abc"},
			want: "abc abc",
		},
		{
			name: "single head",
			args: args{"#abc abc abc"},
			want: "abc abc",
		},
		{
			name: "multiple",
			args: args{"#abc abc #xxxx #xxx#cccc abc #ddd"},
			want: "abc abc",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanLabelText(tt.args.text); got != tt.want {
				t.Errorf("CleanLabelText() = %v, want %v", got, tt.want)
			}
		})
	}
}
