@echo off
rem Pure ASCII on purpose: non-ASCII bytes get mangled by the console code
rem page and turn into bogus commands.
setlocal EnableDelayedExpansion
cd /d "%~dp0"
title LM Studio GGUF Downloader

call :findpy
if not defined PY goto nopython

:menu
cls
echo ==================================================
echo   LM Studio GGUF Downloader
echo   Python: !PY!
echo ==================================================
echo.
echo   [1] ModelScope  (FAST, recommended, about 8.5 MB/s)
echo   [2] hf-mirror   (slower, about 0.7 MB/s, more models)
echo   [0] Exit
echo.
set "CHOICE="
set /p "CHOICE=Choose 1 / 2 / 0: "

if "!CHOICE!"=="0" exit /b 0
if "!CHOICE!"=="1" goto modelscope
if "!CHOICE!"=="2" goto hfmirror
goto menu

:modelscope
set "SCRIPT=modelscope_download.py"
goto askrepo

:hfmirror
set "SCRIPT=hf_download.py"
goto askrepo

:askrepo
cls
echo ==================================================
echo   Step 1 of 2 - repo name
echo ==================================================
echo.
echo Format:  publisher/model-name
echo Example: lmstudio-community/Qwen2.5-7B-Instruct-GGUF
echo          FPHam/Clever_Sydney-4_12b_GGUF
echo.
echo Optional: paste the full URL, it will be cleaned up.
echo   https://huggingface.co/lmstudio-community/Qwen2.5-7B-Instruct-GGUF
echo   https://modelscope.cn/models/lmstudio-community/Qwen2.5-7B-Instruct-GGUF
echo.
set "REPO="
set /p "REPO=Repo: "
if "!REPO!"=="" goto menu

rem --- clean the repo input: URLs, tree/main suffix, spaces, quotes
set "REPO=!REPO:https://huggingface.co/=!"
set "REPO=!REPO:http://huggingface.co/=!"
set "REPO=!REPO:https://hf-mirror.com/=!"
set "REPO=!REPO:http://hf-mirror.com/=!"
set "REPO=!REPO:https://modelscope.cn/models/=!"
set "REPO=!REPO:http://modelscope.cn/models/=!"
set "REPO=!REPO:https://www.modelscope.cn/models/=!"
set "REPO=!REPO:""=!"
rem drop any /tree/... or /resolve/... tail
for /f "tokens=1,2 delims=/" %%A in ("!REPO!") do set "REPO=%%A/%%B"
if /i "!REPO:~-5!"=="/tree" for /f "tokens=1 delims=/" %%A in ("!REPO!") do set "REPO=%%A"
echo.
echo [i] using repo: !REPO!

echo.
echo ==================================================
echo   Step 2 of 2 - what to download
echo ==================================================
echo.
echo   [1] List files only
echo   [2] Download one file
echo   [3] Download ALL gguf files
echo   [0] Back
echo.
set "ACT="
set /p "ACT=Choose 1/2/3/0 [1]: "
if "!ACT!"=="" set "ACT=1"
if "!ACT!"=="0" goto menu
if "!ACT!"=="3" goto dl_all

echo.
"!PY!" "%~dp0!SCRIPT!" --repo "!REPO!" --list
if errorlevel 1 goto pause_menu
if "!ACT!"=="1" goto pause_menu

echo.
echo ==================================================
echo   Which file?
echo ==================================================
echo.
echo Type the NUMBER in brackets from the list above:
echo    2        one file
echo    1,3      these two
echo    1-3      a range
echo.
echo You can also type a keyword instead of a number:
echo    q4_k_m   q6_k   q8_0     (case-insensitive, partial is fine)
echo.
echo Leave it empty to go back.
echo.
set "FILE="
set /p "FILE=Number or keyword: "
if "!FILE!"=="" goto pause_menu
echo.
"!PY!" "%~dp0!SCRIPT!" --repo "!REPO!" --file "!FILE!"
goto pause_menu

:dl_all
echo.
"!PY!" "%~dp0!SCRIPT!" --repo "!REPO!" --all
goto pause_menu

:pause_menu
echo.
echo --------------------------------------------------
echo Done. Press Enter to go back to the menu.
pause >nul
goto menu

rem ================================================ subroutines
:findpy
set "PY="
for /f "delims=" %%P in ('where python 2^>nul') do call :trypy "%%~fP"
if defined PY goto :eof
if exist "%LOCALAPPDATA%\Python\bin\python.exe" set "PY=%LOCALAPPDATA%\Python\bin\python.exe"
if defined PY goto :eof
for /d %%D in ("%LOCALAPPDATA%\Python\pythoncore-*") do call :trypy "%%~fD\python.exe"
if defined PY goto :eof
call :trypy "C:\Python312\python.exe"
call :trypy "C:\Python311\python.exe"
goto :eof

:trypy
if defined PY goto :eof
if not exist %1 goto :eof
for %%S in (%1) do if %%~zS GTR 1024 set "PY=%~f1"
goto :eof

:nopython
echo [x] Python 3 was not found.
echo     Install it with:  winget install -e --id Python.Python.3.12
echo.
pause
exit /b 2
