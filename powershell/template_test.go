package powershell

import (
	"bytes"
	"testing"
)

func TestEscapeQuotesOfCommandLineTemplate(t *testing.T) {
	t.Parallel()

	command := `& { if (Test-Path variable:global:ProgressPreference){$ProgressPreference='SilentlyContinue'};;&"C:/Windows/Temp/Test.ps1";exit $LastExitCode }`

	var executePowershellFromCommandLineTemplateRendered bytes.Buffer
	err := executePowershellFromCommandLineTemplate.Execute(&executePowershellFromCommandLineTemplateRendered, executePowershellFromCommandLineTemplateOptions{
		Powershell: command,
	})

	if err != nil {
		t.Errorf("Unable to render command line template: %s", err.Error())
	}

	commandLine := executePowershellFromCommandLineTemplateRendered.String()

	if commandLine != `powershell -NoProfile -ExecutionPolicy Bypass "& { if (Test-Path variable:global:ProgressPreference){$ProgressPreference='SilentlyContinue'};;&\"C:/Windows/Temp/Test.ps1\";exit $LastExitCode }"` {
		t.Errorf("Command line template output not as expected: %s", err.Error())
	}
}

func TestDeleteFileTemplate_IsIdempotent(t *testing.T) {
	t.Parallel()

	var rendered bytes.Buffer
	err := deleteFileTemplate.Execute(&rendered, deleteFileTemplateOptions{FilePath: `C:/Temp/test.iso`})
	if err != nil {
		t.Fatalf("unable to render delete file template: %v", err)
	}

	got := rendered.String()
	if got == "" {
		t.Fatal("expected delete file template output, got empty string")
	}
	if !bytes.Contains([]byte(got), []byte("exit 0")) {
		t.Fatalf("expected delete file template to succeed when the path is missing or removed already, got: %s", got)
	}
	if bytes.Contains([]byte(got), []byte("$LastExitCode")) {
		t.Fatalf("delete file template should not rely on stale $LastExitCode, got: %s", got)
	}
}
