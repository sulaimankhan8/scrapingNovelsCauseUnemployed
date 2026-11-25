package lang

import "testing"

func TestDetect(t *testing.T) {
	cases := []struct{ text, want string }{
		{"“咦？”希恩有些意外。整个局域魔网的名称是霍格沃茨，但他并没有考虑能否修改的问题。", Chinese},
		{"眼前的場景是那樣的普通，以至於巴頓沒覺得有任何問題。雖然他隱約覺得有一點熟悉", Chinese},
		{"俺の名前は向田剛志。ちなみに２７歳独身だ。日本の地方都市に住むしがないサラリーマンだった。", Japanese},
		{"<p>国内有数のS級クラン《黎明》。その装備管理を十年間にわたって一人で引き受けてきた男</p>", Japanese},
		{"김독자는 평범한 회사원이었다. 그가 십 년 동안 읽어 온 웹소설의 결말을 아는 유일한 독자였다.", Korean},
		{"<p>Klein woke up in a dark room. The Tarot Club would meet again on Monday afternoon.</p>", English},
		{"Chapter 3: 诡秘之主 — Klein Moretti met Audrey Hall at the Tarot Club gathering today.", English},
		{"第一章", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := Detect(c.text); got != c.want {
			t.Errorf("Detect(%.30q) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestNorm(t *testing.T) {
	for in, want := range map[string]string{"zh-TW": Chinese, "zh": Chinese, "JA": Japanese, "ko-KR": Korean, "": English, "en": English} {
		if got := Norm(in); got != want {
			t.Errorf("Norm(%q) = %q, want %q", in, got, want)
		}
	}
	if NeedsTranslation("") || NeedsTranslation("en") || !NeedsTranslation("zh") {
		t.Error("NeedsTranslation")
	}
}
