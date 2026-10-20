package menu

import "testing"

func TestCategoryValid(t *testing.T) {
	tests := []struct {
		category Category
		want     bool
	}{
		{CategoryFood, true},
		{CategoryDrink, true},
		{"", false},
		{"dessert", false},
		// 大文字小文字は区別する。DB の CHECK 制約も小文字しか受け付けない。
		{"Food", false},
	}

	for _, tt := range tests {
		if got := tt.category.Valid(); got != tt.want {
			t.Errorf("Category(%q).Valid() = %v, 期待値 %v", tt.category, got, tt.want)
		}
	}
}
