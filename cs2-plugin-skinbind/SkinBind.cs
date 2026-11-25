using System.Security.Cryptography;
using System.Text.Json;
using CounterStrikeSharp.API;
using CounterStrikeSharp.API.Core;
using CounterStrikeSharp.API.Core.Attributes.Registration;
using CounterStrikeSharp.API.Modules.Commands;
using CounterStrikeSharp.API.Modules.Timers;
using CounterStrikeSharp.API.Modules.Utils;
using MySqlConnector;

namespace SkinBind;

public class SkinBindConfig
{
    public string WelcomeTitle { get; set; } = "欢迎来到 Sherry 的专属服务器！";
    public bool JoinBroadcast { get; set; } = true;
    public bool AutoSendBindCode { get; set; } = true;
    public int CodeExpireMinutes { get; set; } = 10;
    public float JoinDelaySeconds { get; set; } = 3.0f;
}

public class SkinBindPlugin : BasePlugin
{
    public override string ModuleName => "SkinBind";
    public override string ModuleVersion => "1.2.0";
    public override string ModuleAuthor => "CS2 Panel";

    private string _connStr = "";
    private string _webUrl = "http://127.0.0.1:8080";
    private SkinBindConfig _cfg = new();

    public override void Load(bool hotReload)
    {
        LoadConfigs();
        RegisterEventHandler<EventPlayerConnectFull>(OnPlayerConnectFull);
    }

    private void LoadConfigs()
    {
        try
        {
            var sbPath = Path.Combine(Server.GameDirectory, "csgo", "addons", "counterstrikesharp", "configs", "plugins", "SkinBind", "SkinBind.json");
            if (File.Exists(sbPath))
            {
                var parsed = JsonSerializer.Deserialize<SkinBindConfig>(File.ReadAllText(sbPath));
                if (parsed != null) _cfg = parsed;
            }
        }
        catch (Exception ex)
        {
            Console.WriteLine($"[SkinBind] Failed to load SkinBind.json: {ex.Message}");
        }

        try
        {
            var wpPath = Path.Combine(Server.GameDirectory, "csgo", "addons", "counterstrikesharp", "configs", "plugins", "WeaponPaints", "WeaponPaints.json");
            if (!File.Exists(wpPath)) return;
            using var doc = JsonDocument.Parse(File.ReadAllText(wpPath));
            var root = doc.RootElement;
            var host = root.GetProperty("DatabaseHost").GetString() ?? "127.0.0.1";
            var port = root.GetProperty("DatabasePort").GetInt32();
            var user = root.GetProperty("DatabaseUser").GetString() ?? "wpuser";
            var pass = root.GetProperty("DatabasePassword").GetString() ?? "";
            var name = root.GetProperty("DatabaseName").GetString() ?? "cs2_weaponpaints";
            _connStr = $"Server={host};Port={port};Database={name};Uid={user};Pwd={pass};SslMode=None;";
            if (root.TryGetProperty("Website", out var w) && !string.IsNullOrWhiteSpace(w.GetString()))
            {
                _webUrl = w.GetString()!.TrimEnd('/');
            }
        }
        catch (Exception ex)
        {
            Console.WriteLine($"[SkinBind] Failed to load WeaponPaints.json: {ex.Message}");
        }
    }

    private HookResult OnPlayerConnectFull(EventPlayerConnectFull @event, GameEventInfo info)
    {
        var player = @event.Userid;
        if (player == null || !player.IsValid || player.IsBot || player.IsHLTV)
            return HookResult.Continue;

        var steamId = player.SteamID;
        var playerName = player.PlayerName;

        if (_cfg.JoinBroadcast)
        {
            Server.PrintToChatAll($" {ChatColors.Gold}★{ChatColors.Default} 玩家 {ChatColors.LightBlue}{playerName}{ChatColors.Default} 加入了服务器，{_cfg.WelcomeTitle}");
        }

        var delay = _cfg.JoinDelaySeconds > 0.5f ? _cfg.JoinDelaySeconds : 3.0f;
        AddTimer(delay, () =>
        {
            var p = Utilities.GetPlayerFromSteamId(steamId);
            if (p == null || !p.IsValid || p.IsBot) return;

            p.PrintToChat($" {ChatColors.Gold}========================================");
            p.PrintToChat($" {ChatColors.Green}★ {ChatColors.Gold}{_cfg.WelcomeTitle}");
            p.PrintToChat($" {ChatColors.LightBlue}网页换肤保存即自动刷新（无需手动输入 !wp）{ChatColors.Default} | {ChatColors.LightBlue}!bind{ChatColors.Default} 重新获取配对码 | {ChatColors.LightBlue}!knife{ChatColors.Default} 切刀");
            p.PrintToChat($" {ChatColors.Gold}========================================");

            if (_cfg.AutoSendBindCode)
            {
                IssueAndSendBindCode(p, isAuto: true);
            }
        });

        return HookResult.Continue;
    }

    [ConsoleCommand("css_bind", "Generate a one-time 6-digit pairing code for the web skin workshop")]
    [ConsoleCommand("css_skin", "Alias for css_bind")]
    [CommandHelper(whoCanExecute: CommandUsage.CLIENT_ONLY)]
    public void OnBindCommand(CCSPlayerController? player, CommandInfo info)
    {
        if (player == null || !player.IsValid || player.IsBot) return;
        IssueAndSendBindCode(player, isAuto: false);
    }

    [ConsoleCommand("css_skinbind_synced", "Notify an online player that their web skins were auto-refreshed")]
    [CommandHelper(minArgs: 1, usage: "<steamid64>", whoCanExecute: CommandUsage.SERVER_ONLY)]
    public void OnSyncedNotifyCommand(CCSPlayerController? caller, CommandInfo info)
    {
        var raw = info.GetArg(1).Trim();
        if (!ulong.TryParse(raw, out var steamId)) return;
        Server.NextFrame(() =>
        {
            var p = Utilities.GetPlayerFromSteamId(steamId);
            if (p != null && p.IsValid && !p.IsBot)
            {
                p.PrintToChat($" {ChatColors.Green}[皮肤工坊]{ChatColors.Default} 网页保存的新皮肤已{ChatColors.Gold}自动同步生效{ChatColors.Default}！（无需输入 !wp）");
            }
        });
    }

    private void IssueAndSendBindCode(CCSPlayerController player, bool isAuto)
    {
        if (string.IsNullOrEmpty(_connStr)) LoadConfigs();
        if (string.IsNullOrEmpty(_connStr))
        {
            player.PrintToChat($" {ChatColors.Red}[皮肤工坊]{ChatColors.Default} 数据库配置未就绪，请联系管理员。");
            return;
        }

        var steamIdUlong = player.SteamID;
        var steamId = steamIdUlong.ToString();
        var code = RandomNumberGenerator.GetInt32(100000, 999999).ToString();
        var token = Convert.ToHexString(RandomNumberGenerator.GetBytes(18)).ToLowerInvariant();
        var webUrl = _webUrl;
        var ttlMin = _cfg.CodeExpireMinutes > 0 ? _cfg.CodeExpireMinutes : 10;

        Task.Run(async () =>
        {
            try
            {
                await using var conn = new MySqlConnection(_connStr);
                await conn.OpenAsync();
                await using var cmd = conn.CreateCommand();
                cmd.CommandText = @"
                    CREATE TABLE IF NOT EXISTS wp_web_tokens (
                        token VARCHAR(64) NOT NULL PRIMARY KEY,
                        code VARCHAR(16) NOT NULL,
                        steamid VARCHAR(20) NOT NULL,
                        expires_at DATETIME NOT NULL,
                        used TINYINT(1) NOT NULL DEFAULT 0,
                        INDEX idx_code (code),
                        INDEX idx_exp (expires_at)
                    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
                    DELETE FROM wp_web_tokens WHERE expires_at < UTC_TIMESTAMP() OR used = 1;
                    INSERT INTO wp_web_tokens (token, code, steamid, expires_at, used)
                    VALUES (@tok, @code, @sid, DATE_ADD(UTC_TIMESTAMP(), INTERVAL @ttl MINUTE), 0);";
                cmd.Parameters.AddWithValue("@tok", token);
                cmd.Parameters.AddWithValue("@code", code);
                cmd.Parameters.AddWithValue("@sid", steamId);
                cmd.Parameters.AddWithValue("@ttl", ttlMin);
                await cmd.ExecuteNonQueryAsync();

                Server.NextFrame(() =>
                {
                    var p = Utilities.GetPlayerFromSteamId(steamIdUlong);
                    if (p != null && p.IsValid)
                    {
                        var prefix = isAuto ? "进服自动配对码" : "你的 6 位配对码";
                        p.PrintToChat($" {ChatColors.Green}[皮肤工坊]{ChatColors.Default} {prefix}：{ChatColors.Gold}{code}{ChatColors.Default}（{ttlMin} 分钟内有效，仅你可见）");
                        p.PrintToChat($" {ChatColors.Green}[皮肤工坊]{ChatColors.Default} 换肤网页：{ChatColors.LightBlue}{webUrl}{ChatColors.Default}（保存即自动刷新）");
                    }
                });
            }
            catch (Exception ex)
            {
                Console.WriteLine($"[SkinBind] DB error: {ex.Message}");
                Server.NextFrame(() =>
                {
                    var p = Utilities.GetPlayerFromSteamId(steamIdUlong);
                    if (p != null && p.IsValid)
                        p.PrintToChat($" {ChatColors.Red}[皮肤工坊]{ChatColors.Default} 生成配对码失败，请稍后重试。");
                });
            }
        });
    }
}
