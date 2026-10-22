package order

import "testing"

func TestCheckTotalYen(t *testing.T) {
	c := Check{
		Orders: []Order{
			{Items: []Item{
				{Name: "生ビール", PriceYen: 600, Quantity: 2},
				{Name: "枝豆", PriceYen: 380, Quantity: 1},
			}},
			{Items: []Item{
				{Name: "唐揚げ", PriceYen: 680, Quantity: 1},
			}},
		},
	}

	if got, want := c.Orders[0].TotalYen(), 600*2+380; got != want {
		t.Errorf("注文 1 回目の金額 = %d, 期待値 %d", got, want)
	}
	if got, want := c.TotalYen(), 600*2+380+680; got != want {
		t.Errorf("伝票の合計 = %d, 期待値 %d", got, want)
	}
}

func TestCheckTotalYenWithoutOrders(t *testing.T) {
	// 入店直後の伝票は注文がなく、合計は 0 円になる。
	if got := (Check{}).TotalYen(); got != 0 {
		t.Errorf("合計 = %d, 期待値 0", got)
	}
}

func TestStatusValid(t *testing.T) {
	for _, s := range []Status{StatusOpen, StatusPaid, StatusClosed} {
		if !s.Valid() {
			t.Errorf("Status(%q).Valid() = false, 期待値 true", s)
		}
	}
	for _, s := range []Status{"", "cancelled", "Open"} {
		if s.Valid() {
			t.Errorf("Status(%q).Valid() = true, 期待値 false", s)
		}
	}
}

func TestValidateTableNo(t *testing.T) {
	if err := ValidateTableNo(1); err != nil {
		t.Errorf("ValidateTableNo(1) = %v, 期待値 nil", err)
	}
	for _, n := range []int{0, -1} {
		if err := ValidateTableNo(n); err == nil {
			t.Errorf("ValidateTableNo(%d) = nil, エラーを期待", n)
		}
	}
}

func TestValidateLines(t *testing.T) {
	tests := []struct {
		name    string
		lines   []Line
		wantErr bool
	}{
		{"妥当な注文", []Line{{MenuItemID: 1, Quantity: 2}}, false},
		{"品目なし", nil, true},
		{"識別子が 0", []Line{{MenuItemID: 0, Quantity: 1}}, true},
		{"数量が 0", []Line{{MenuItemID: 1, Quantity: 0}}, true},
		{"2 品目めだけ不正", []Line{{MenuItemID: 1, Quantity: 1}, {MenuItemID: 2, Quantity: -1}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLines(tt.lines)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLines() = %v, エラーを期待するか: %v", err, tt.wantErr)
			}
		})
	}
}
