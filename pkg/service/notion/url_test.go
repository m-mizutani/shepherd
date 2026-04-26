package notion_test

import (
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/service/notion"
)

func TestParseURL(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantType  types.NotionObjectType
		wantID    string
		expectErr bool
	}{
		{
			name:     "page with slug",
			input:    "https://www.notion.so/myws/Project-Plan-1f2e3d4c5b6a7980abcd1234567890ef",
			wantType: types.NotionObjectPage,
			wantID:   "1f2e3d4c5b6a7980abcd1234567890ef",
		},
		{
			name:     "database with view query",
			input:    "https://www.notion.so/myws/abcdef0123456789abcdef0123456789?v=cafef00dcafef00dcafef00dcafef00d",
			wantType: types.NotionObjectDatabase,
			wantID:   "abcdef0123456789abcdef0123456789",
		},
		{
			name:     "hyphenated UUID bare",
			input:    "1f2e3d4c-5b6a-7980-abcd-1234567890ef",
			wantType: types.NotionObjectPage,
			wantID:   "1f2e3d4c5b6a7980abcd1234567890ef",
		},
		{
			name:     "32 hex bare",
			input:    "1F2E3D4C5B6A7980ABCD1234567890EF",
			wantType: types.NotionObjectPage,
			wantID:   "1f2e3d4c5b6a7980abcd1234567890ef",
		},
		{
			name:      "empty",
			input:     "",
			expectErr: true,
		},
		{
			name:      "non-notion host",
			input:     "https://example.com/foo",
			expectErr: true,
		},
		{
			name:      "no id in path",
			input:     "https://www.notion.so/myws/no-id-here",
			expectErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ot, id, err := notion.ParseURL(tc.input)
			if tc.expectErr {
				gt.Error(t, err)
				return
			}
			gt.NoError(t, err)
			gt.Equal(t, ot, tc.wantType)
			gt.Equal(t, id, tc.wantID)
		})
	}
}
