' PullAI Web UI Launcher
' Avvia il server senza finestra visibile, poi apre il browser
Set oShell = CreateObject("WScript.Shell")

' Percorso di pullai.exe nella stessa cartella di questo script
Dim sDir
sDir = CreateObject("Scripting.FileSystemObject").GetParentFolderName(WScript.ScriptFullName)

Dim sExe
sExe = sDir & "\pullai.exe"

' Avvia server in background (windowStyle=0 = nascosto)
oShell.Run Chr(34) & sExe & Chr(34) & " serve --port 11500", 0, False

' Aspetta che il server sia pronto
WScript.Sleep 1500

' Apri il browser
oShell.Run "http://localhost:11500", 1, False
