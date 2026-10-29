package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestFormReopenPreservesFieldsAndClearedPassword(t *testing.T) {
	m := modelFormAt(t, "password")
	updated, _ := m.Update(tea.PasteMsg{Content: "secret-value"})
	form := updated.(model).form
	form.clearPassword()
	reopened := reopenConnectionForm(form)
	if reopened.values.Name != "nas" || reopened.values.Host != "nas.local" {
		t.Fatal("reopening lost connection fields")
	}
	if reopened.values.Password != "" || reopened.values.PasswordConfirmation != "" {
		t.Fatal("reopening restored cleared credentials")
	}
	if _, err := reopened.SecretChange(); err == nil {
		t.Fatal("reopened form accepted a cleared password")
	}
}

func TestFormDisposeClearsBoundPasswordInputs(t *testing.T) {
	m := modelFormAt(t, "password")
	updated, _ := m.Update(tea.PasteMsg{Content: "secret-value"})
	form := updated.(model).form
	password := form.passwordInput
	confirmation := form.passwordConfirmationInput
	form.dispose()
	if password.GetValue() != "" || confirmation.GetValue() != "" {
		t.Fatal("disposed form retained credentials in input widgets")
	}
	if form.Completed() || form.View() != "" {
		t.Fatal("disposed form remains active")
	}
	form.dispose()
	reopened := reopenConnectionForm(form)
	if reopened.values.Name != "nas" || reopened.values.Password != "" {
		t.Fatal("disposed form lost fields or restored credentials on reopening")
	}
}

func TestZeroFormDispose(t *testing.T) {
	var form connectionForm
	form.dispose()
	form.dispose()
	if form.Completed() || form.View() != "" {
		t.Fatal("zero form became active after disposal")
	}
}

func TestFormCancelClearsCredentialsAcrossCopies(t *testing.T) {
	m := modelFormAt(t, "password")
	form, _ := m.form.Update(tea.PasteMsg{Content: "secret-value"})
	copy := form
	copy.values.PasswordConfirmation = "secret-value"
	copy.passwordConfirmationInput.Value(&copy.values.PasswordConfirmation)
	copy.form.CancelCmd()
	if form.values.Password != "" || form.values.PasswordConfirmation != "" {
		t.Fatal("cancel retained credentials in a form copy")
	}
	if form.passwordInput.GetValue() != "" || form.passwordConfirmationInput.GetValue() != "" {
		t.Fatal("cancel retained credentials in input widgets")
	}
}

func TestFormInputBindingSurvivesCopyAndSanitization(t *testing.T) {
	m := modelFormAt(t, "password")
	form := m.form
	copy := form
	copy.values.Name = "nas\x1b"
	copy.View()
	if form.values.Name != "nas\\x1B" {
		t.Fatalf("sanitized name = %q", form.values.Name)
	}
	updated, _ := copy.Update(tea.PasteMsg{Content: "secret-value"})
	if form.values.Password != "secret-value" || updated.values.Password != "secret-value" {
		t.Fatal("input binding did not survive copying and sanitization")
	}
}
