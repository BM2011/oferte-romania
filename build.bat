@echo off
go build -trimpath -ldflags "-s -w" -o OferteRomania.exe .
if %errorlevel%==0 (echo Gata: OferteRomania.exe) else (echo Compilarea a esuat. Ai Go instalat? https://go.dev/dl/)
pause
