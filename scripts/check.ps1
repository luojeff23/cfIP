Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$projectRoot = Split-Path -Parent $PSScriptRoot
$runBatPath = Join-Path $projectRoot "run.bat"

Push-Location $projectRoot
try {
    Write-Host "[1/4] verify run.bat entrypoint" -ForegroundColor Cyan
    $runBatContent = Get-Content $runBatPath -Raw
    if ($runBatContent -notmatch '(?im)^\s*go run \.\s*$') {
        throw "run.bat must start the full package with 'go run .'"
    }

    Write-Host "[2/4] go test ./..." -ForegroundColor Cyan
    go test ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Host "CHECK FAILED: tests did not pass." -ForegroundColor Red
        exit $LASTEXITCODE
    }

    Write-Host "[3/4] go build ./..." -ForegroundColor Cyan
    go build ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Host "CHECK FAILED: build did not pass." -ForegroundColor Red
        exit $LASTEXITCODE
    }

    Write-Host "[4/4] startup smoke test" -ForegroundColor Cyan
    $smokeBinary = Join-Path $env:TEMP ("cfping-smoke-{0}.exe" -f $PID)
    go build -o $smokeBinary .
    if ($LASTEXITCODE -ne 0) {
        Write-Host "CHECK FAILED: smoke binary build did not pass." -ForegroundColor Red
        exit $LASTEXITCODE
    }

    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $smokeBinary
    $psi.WorkingDirectory = $projectRoot
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.EnvironmentVariables["CFPING_SKIP_BROWSER"] = "1"
    $proc = [System.Diagnostics.Process]::Start($psi)

    try {
        $ready = $false
        for ($i = 0; $i -lt 20; $i++) {
            Start-Sleep -Milliseconds 500
            try {
                $resp = Invoke-WebRequest -UseBasicParsing http://localhost:13334
                if ($resp.StatusCode -eq 200) {
                    $ready = $true
                    break
                }
            } catch {
            }
        }

        if (-not $ready) {
            $stdout = $proc.StandardOutput.ReadToEnd()
            $stderr = $proc.StandardError.ReadToEnd()
            throw "startup smoke test failed. stdout: $stdout stderr: $stderr"
        }
    }
    finally {
        if ($proc -and -not $proc.HasExited) {
            $proc.Kill()
            $proc.WaitForExit()
        }
        if (Test-Path $smokeBinary) {
            Remove-Item $smokeBinary -Force
        }
    }

    Write-Host "CHECK PASSED" -ForegroundColor Green
}
finally {
    Pop-Location
}
