using System;
using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using System.Collections.Generic;

// Only used on a disposable runner. Hold an ordinary exclusive file handle
// after the staged manifest reader exits; do not change the installer or ACLs.
public sealed class InstallerProbe : IDisposable {
    private readonly CancellationTokenSource stop = new CancellationTokenSource();
    private readonly Task watcher;
    public volatile bool ObservedManifest, Locked;

    public InstallerProbe(string temp) {
        watcher = Task.Run(() => Watch(temp));
    }
    private void Watch(string temp) {
        while (!stop.IsCancellationRequested) {
            foreach (var process in Process.GetProcessesByName("ACU Runtime")) {
                using (process) {
                    string path;
                    try { path = process.MainModule.FileName; }
                    catch (InvalidOperationException) { continue; }
                    catch (System.ComponentModel.Win32Exception) { continue; }
                    var dir = Path.GetDirectoryName(path);
                    if (!String.Equals(Path.GetDirectoryName(dir), temp, StringComparison.OrdinalIgnoreCase) ||
                        !Path.GetFileName(dir).StartsWith("acu-setup-", StringComparison.OrdinalIgnoreCase)) continue;
                    ObservedManifest = true;
                    var handle = process.SafeHandle;
                    uint exitCode;
                    if (!process.WaitForExit(10000) || !GetExitCodeProcess(handle, out exitCode) || exitCode != 0)
                        throw new Exception("The first staged process did not exit successfully");
                    using (var locked = File.Open(path, FileMode.Open, FileAccess.Read, FileShare.None)) {
                        Locked = true;
                        stop.Token.WaitHandle.WaitOne(90000);
                    }
                    return;
                }
            }
            Thread.Sleep(1);
        }
    }
    public void Dispose() {
        stop.Cancel();
        watcher.GetAwaiter().GetResult();
        stop.Dispose();
    }

    private delegate bool Callback(IntPtr hwnd, IntPtr data);
    [DllImport("kernel32.dll")] private static extern bool GetExitCodeProcess(Microsoft.Win32.SafeHandles.SafeProcessHandle handle, out uint code);
    [DllImport("user32.dll")] private static extern bool EnumChildWindows(IntPtr hwnd, Callback callback, IntPtr data);
    [DllImport("user32.dll", CharSet=CharSet.Unicode)] private static extern int GetClassName(IntPtr hwnd, StringBuilder text, int count);
    [DllImport("user32.dll", EntryPoint="GetWindowLongW")] private static extern int Style(IntPtr hwnd, int index);
    [DllImport("user32.dll", CharSet=CharSet.Unicode)] private static extern IntPtr SendMessageTimeout(IntPtr hwnd, uint msg, IntPtr max, StringBuilder text, uint flags, uint timeout, out IntPtr result);
    [DllImport("user32.dll")] private static extern bool PostMessage(IntPtr hwnd, uint msg, IntPtr w, IntPtr l);

    private static IntPtr okButton;
    public static string ReadDialogBody(IntPtr dialog) {
        var lines = new List<string>();
        var controls = new List<string>();
        bool unreadable = false;
        okButton = IntPtr.Zero;
        EnumChildWindows(dialog, (h, _) => {
            var cls = new StringBuilder(128);
            GetClassName(h, cls, cls.Capacity);
            var kind = cls.ToString();
            if (kind == "Button" || (kind == "Static" && (Style(h, -16) & 31) != 3)) {
                var text = new StringBuilder(8192);
                IntPtr result;
                if (SendMessageTimeout(h, 13, (IntPtr)text.Capacity, text, 2, 2000, out result) == IntPtr.Zero)
                    unreadable = true;
                controls.Add(kind + "='" + text + "'");
                if (kind == "Static") lines.Add(text.ToString());
                else if (String.Equals(text.ToString().Replace("&", ""), "OK", StringComparison.OrdinalIgnoreCase) || text.ToString() == "确定") okButton = h;
            } else controls.Add(kind);
            return true;
        }, IntPtr.Zero);
        Console.WriteLine("Dialog controls: " + String.Join(", ", controls));
        if (okButton == IntPtr.Zero || unreadable || lines.Count == 0) throw new Exception("Dialog controls not yet readable");
        return String.Join("\n", lines);
    }
    public static void Accept(IntPtr dialog) {
        PostMessage(okButton, 0x00f5, IntPtr.Zero, IntPtr.Zero); // BM_CLICK
    }
}
