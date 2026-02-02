@echo off
cheatcolor 0A
echo.
echo ========================================
echo    CREATIVE SERVICE - SETUP GITHUB
echo ========================================
echo.
echo Escolha uma opcao:
echo.
echo [1] Setup Completo (Husky + Git + Branches)
echo [2] Apenas Setup Husky
echo [3] Apenas Criar Branches
echo [4] Ver Guia Rapido
echo [5] Ver Documentacao Completa
echo [6] Sair
echo.
set /p opcao="Digite o numero da opcao: "

if "%opcao%"=="1" goto setup_completo
if "%opcao%"=="2" goto setup_husky
if "%opcao%"=="3" goto criar_branches
if "%opcao%"=="4" goto guia_rapido
if "%opcao%"=="5" goto documentacao
if "%opcao%"=="6" goto sair

:setup_completo
echo.
echo [*] Executando setup completo...
powershell -ExecutionPolicy Bypass -File setup-github.ps1
if %errorlevel% neq 0 (
    echo [ERRO] Falha no setup do Husky
    pause
    exit /b 1
)
echo.
echo [*] Criando branches...
powershell -ExecutionPolicy Bypass -File run-git-setup-branches.ps1
echo.
echo [OK] Setup completo finalizado!
pause
exit /b 0

:setup_husky
echo.
echo [*] Configurando Husky e Commitlint...
powershell -ExecutionPolicy Bypass -File setup-github.ps1
echo.
echo [OK] Husky configurado!
pause
exit /b 0

:criar_branches
echo.
echo [*] Criando estrutura de branches...
powershell -ExecutionPolicy Bypass -File run-git-setup-branches.ps1
echo.
echo [OK] Branches criadas!
pause
exit /b 0

:guia_rapido
echo.
echo [*] Abrindo guia rapido...
start QUICK_START_GIT.md
pause
exit /b 0

:documentacao
echo.
echo [*] Abrindo documentacao completa...
start SETUP_GITHUB.md
pause
exit /b 0

:sair
echo.
echo Ate logo!
exit /b 0
