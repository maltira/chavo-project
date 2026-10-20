package email

import (
	"bytes"
	"strings"
	"testing"
)

func render(t *testing.T, path string, data any) string {
	t.Helper()
	var body bytes.Buffer
	if err := mustParseTemplate(path).Execute(&body, data); err != nil {
		t.Fatalf("render %s: %v", path, err)
	}
	return body.String()
}

func assertContains(t *testing.T, html string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(html, p) {
			t.Errorf("letter must contain %q", p)
		}
	}
}

// ни одно письмо не должно содержать неразобранных действий шаблона или пустых мест
func assertWhole(t *testing.T, html string) {
	t.Helper()
	for _, bad := range []string{"{{", "}}", "<no value>", "&lt;no value&gt;"} {
		if strings.Contains(html, bad) {
			t.Errorf("letter contains %q", bad)
		}
	}
	assertContains(t, html,
		"Chavo", "Lora", "#141413", "#1b1b1a", // палитра сервиса
		`src="cid:logo-on-dark.png"`)
	// только тёмная тема: светлой палитры и переключения по схеме нет
	for _, bad := range []string{"prefers-color-scheme", "#faf9f5 !important", "#f0eee6"} {
		if strings.Contains(html, bad) {
			t.Errorf("letter must be dark only, found %q", bad)
		}
	}
}

func TestVerificationLetter(t *testing.T) {
	url := "http://localhost:3000/auth/register/verify?token=abc&x=1"
	html := render(t, "templates/verification.html", VerificationData{VerifyURL: url, ExpiresIn: "15 минут"})
	assertWhole(t, html)
	// & в ссылке экранируется как в атрибуте, а не теряется
	assertContains(t, html, "#d97757", "Добро пожаловать в Chavo", "Подтвердить email",
		"http://localhost:3000/auth/register/verify?token=abc&amp;x=1", "в течение", "15 минут")
}

func TestPasswordResetLetter(t *testing.T) {
	html := render(t, "templates/reset_pass.html", PassResetData{
		ResetURL: "http://localhost:3000/auth/reset/confirm?token=t0k", ExpiresIn: "15 минут",
	})
	assertWhole(t, html)
	assertContains(t, html, "Сброс пароля", "Сбросить пароль",
		"http://localhost:3000/auth/reset/confirm?token=t0k", "15 минут")
}

func TestOTPLetterShowsEveryDigitInItsOwnCell(t *testing.T) {
	html := render(t, "templates/otp.html", OtpData{
		Code: "402917", Digits: []string{"4", "0", "2", "9", "1", "7"}, ExpiresIn: "5 минут",
	})
	assertWhole(t, html)
	if n := strings.Count(html, `class="digit"`); n != 6 {
		t.Fatalf("digit cells = %d, want 6", n)
	}
	assertContains(t, html, "Код для входа", "Ваш код для входа в Chavo: 402917", "в течение", "5 минут")
}

// логотип вкладывается в письмо: cid-ссылка в теле, PNG в multipart-части с тем же Content-ID
func TestLetterEmbedsTheLogo(t *testing.T) {
	s := &Sender{user: "noreply@chavo.local"}
	m, err := s.compose("user@example.com", "Тема", `<img src="cid:logo-on-dark.png">`)
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	if _, err = m.WriteTo(&raw); err != nil {
		t.Fatal(err)
	}
	// имена заголовков сравниваем без учёта регистра: gomail пишет Content-Id
	letter := strings.ToLower(raw.String())
	assertContains(t, letter, "content-id: <logo-on-dark.png>", "content-type: image/png", "content-transfer-encoding: base64")
}
