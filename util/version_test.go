package util

import "testing"

// TestVersionCompare 使用与 PHP version_compare 一致的语义做表驱动验证，
// 覆盖常规数字、预发布标识、运算符变体以及会导致旧实现 panic 的边界输入。
func TestVersionCompare(t *testing.T) {
	cases := []struct {
		name     string
		ver1     string
		ver2     string
		operator string
		want     bool
	}{
		// 数字比较
		{"数字相等", "1.0", "1.0", "=", true},
		{"数字相等 eq", "1.0", "1.0", "eq", true},
		{"数字相等 == ", "1.0.0", "1.0.0", "==", true},
		{"数字不等 ne", "1.0", "1.0.1", "!=", true},
		{"少段小于", "1.0", "1.0.1", "<", true},
		{"少段不小于", "1.0", "1.0.1", ">", false},
		{"多段大于", "1.0.1", "1.0", ">", true},
		{"多段大于等于", "1.0.1", "1.0", ">=", true},
		// 数字段对齐：结果段仍参与比较（既有行为）
		{"末尾0比较小", "1.0", "1.0.0", "<", true},
		{"末尾0比较2", "1.0.0", "1.0", ">", true},
		{"前导零忽略", "2024.01", "2024.1", "=", true},

		// 预发布标识：dev < alpha < beta < RC < 正式
		{"alpha 小于 beta", "1.0-alpha", "1.0-beta", "<", true},
		{"beta 大于 alpha", "1.0-beta", "1.0-alpha", ">", true},
		{"alpha 小于正式", "1.0-alpha", "1.0", "<", true},
		{"rc 小于正式", "1.0-rc1", "1.0", "<", true},
		{"RC 与 rc 同序", "1.0-RC", "1.0-rc", "=", true},
		{"正式大于 rc", "1.0", "1.0-rc1", ">", true},
		{"dev 小于 alpha", "1.0-dev", "1.0-alpha", "<", true},

		// 非数字与数字混排
		{"beta1 小于 1", "1.0.0-beta1", "1.0.0.1", "<", true},

		// v 前缀（canonicalize 会归并字符边界）
		{"v 前缀", "v1.0.1", "1.0.2", "<", true},

		// 空输入
		{"空串小于任何", "", "1.0", "<", true},
		{"任何大于空串", "1.0", "", ">", true},
		{"空串等于空串", "", "", "=", true},

		// 曾导致 panic 的边界：尾部点产生空段
		{"尾部点 1", "1.", "1.0", "<", true},
		{"尾部点 2", "1.0", "1.", ">", true},
		{"尾部点相等", "1.", "1.", "=", true},
		{"全部点", ".", "0", "<", true},

		// 运算符非法时返回 false
		{"非法运算符", "1.0", "1.0", "xyz", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("VersionCompare(%q, %q, %q) panicked: %v", tc.ver1, tc.ver2, tc.operator, r)
				}
			}()
			if got := VersionCompare(tc.ver1, tc.ver2, tc.operator); got != tc.want {
				t.Errorf("VersionCompare(%q, %q, %q) = %v, want %v", tc.ver1, tc.ver2, tc.operator, got, tc.want)
			}
		})
	}
}

// TestVersionCompareNoPanic 覆盖各种病态输入（重复点、首部点、长段），
// 旧实现会在部分输入上 index out of range panic，这里只要求不 panic。
func TestVersionCompareNoPanic(t *testing.T) {
	inputs := [][2]string{
		{"1..2", "1.2"},
		{"1.0.", "1.0"},
		{".", "0"},
		{"...", "0"},
		{"1.2.3.", "1.2.3"},
		{"a.b", "a"},
		{"1.a", "1"},
		{"", "1.0"},
	}
	for _, p := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("VersionCompare(%q, %q) panicked: %v", p[0], p[1], r)
				}
			}()
			for _, op := range []string{"<", "<=", ">", ">=", "=", "!="} {
				VersionCompare(p[0], p[1], op)
				VersionCompare(p[1], p[0], op)
			}
		}()
	}
}

// TestVersionCompareSymmetric 确保比较在反转两侧后得到对称结果。
// 允许两侧相等（此时 < 和 > 均为 false，但 == 为 true）。
func TestVersionCompareSymmetric(t *testing.T) {
	pairs := [][2]string{
		{"1.2.3", "1.2.4"},
		{"2.0.0", "10.0.0"},
		{"1.0-beta", "1.0"},
		{"1.0.0-alpha", "1.0.0-beta"},
		{"1.", "1.0"},
		{"2024.01", "2024.1"},
	}
	for _, p := range pairs {
		lt := VersionCompare(p[0], p[1], "<")
		gt := VersionCompare(p[0], p[1], ">")
		eq := VersionCompare(p[0], p[1], "==")
		revLt := VersionCompare(p[1], p[0], "<")
		revGt := VersionCompare(p[1], p[0], ">")
		revEq := VersionCompare(p[1], p[0], "==")
		if lt && gt {
			t.Errorf("VersionCompare(%q, %q): < 与 > 不应同时为 true", p[0], p[1])
		}
		if !lt && !gt && !eq {
			t.Errorf("VersionCompare(%q, %q): 既不小于、不大于也不相等", p[0], p[1])
		}
		if lt != revGt || gt != revLt || eq != revEq {
			t.Errorf("VersionCompare(%q, %q) 反转后不对称: < %v > %v == %v | 反向 < %v > %v == %v",
				p[0], p[1], lt, gt, eq, revLt, revGt, revEq)
		}
	}
}
