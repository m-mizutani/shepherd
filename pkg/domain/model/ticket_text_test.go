package model_test

import (
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
)

func TestCanonicalText(t *testing.T) {
	cases := []struct {
		name string
		in   *model.Ticket
		want string
	}{
		{
			name: "all three sections",
			in: &model.Ticket{
				Title:       "DB outage",
				Description: "users cannot log in",
				Conclusion:  "rotated the leaked credentials",
			},
			want: "DB outage\n\nusers cannot log in\n\nrotated the leaked credentials",
		},
		{
			name: "missing conclusion",
			in: &model.Ticket{
				Title:       "DB outage",
				Description: "users cannot log in",
			},
			want: "DB outage\n\nusers cannot log in",
		},
		{
			name: "whitespace and newlines collapsed",
			in: &model.Ticket{
				Title:       "  DB    outage  ",
				Description: "users\n\ncannot\tlog in",
			},
			want: "DB outage\n\nusers cannot log in",
		},
		{
			name: "all empty",
			in:   &model.Ticket{},
			want: "",
		},
		{
			name: "nil ticket",
			in:   nil,
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := model.CanonicalText(tc.in)
			gt.Equal(t, tc.want, got)
		})
	}
}
