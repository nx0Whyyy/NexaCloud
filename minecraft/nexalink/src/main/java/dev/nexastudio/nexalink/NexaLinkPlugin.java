package dev.nexastudio.nexalink;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.StandardCopyOption;
import java.time.Instant;
import org.bukkit.command.Command;
import org.bukkit.command.CommandSender;
import org.bukkit.plugin.java.JavaPlugin;

public final class NexaLinkPlugin extends JavaPlugin {
    @Override
    public void onEnable() {
        getDataFolder().mkdirs();
        writeStatus();
        getServer().getScheduler().runTaskTimerAsynchronously(this, this::writeStatus, 100L, 200L);
        getLogger().info("NexaLink is connected to the local NexaAgent bridge");
    }

    @Override
    public boolean onCommand(CommandSender sender, Command command, String label, String[] args) {
        sender.sendMessage("NexaLink ONLINE - NexaCloud bridge ready");
        return true;
    }

    private void writeStatus() {
        var target = getDataFolder().toPath().resolve("status.json");
        var temporary = getDataFolder().toPath().resolve("status.json.tmp");
        var json = "{\"online\":true,\"players\":" + getServer().getOnlinePlayers().size()
            + ",\"max_players\":" + getServer().getMaxPlayers() + ",\"timestamp\":\"" + Instant.now() + "\"}";
        try {
            Files.writeString(temporary, json, StandardCharsets.UTF_8);
            Files.move(temporary, target, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE);
        } catch (IOException error) {
            getLogger().warning("Unable to publish telemetry: " + error.getMessage());
        }
    }
}
