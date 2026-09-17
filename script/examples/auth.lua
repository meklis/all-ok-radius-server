-- Подключается через script.auth. Вызывается вместо HTTP API при processor: script.
--
-- request:
--   nas_ip, nas_name, device_mac, dhcp_server_name, dhcp_server_id,
--   ip_address, class_id, option.remote_id, option.circuit_id
--
-- Должна вернуть таблицу с ip_address (фиксированный ip) или pool_name (dhcp-пул),
-- опционально lease_time_sec. Если вернуть {error = "..."} - запрос считается отклонённым.
function authorize(request)
    log.debug("authorize: device_mac=" .. request.device_mac .. " nas_ip=" .. request.nas_ip)

    if request.device_mac == "" then
        return { error = "empty device_mac" }
    end

    return {
        pool_name = "default",
        lease_time_sec = 3600,
    }
end
