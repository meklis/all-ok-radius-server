-- Обработчик auth-запросов (script.auth).
-- request.option.remote_id  - мак свитча (формат AA:BB:CC:DD:EE:FF)
-- request.option.circuit_id - circuit-id, hex-строка без "0x"
--
-- Тип оборудования для парсинга circuit_id берётся из db:getDeviceByMac(mac_sw).parse_type.
-- db обязателен при processor: script (см. script.database.devices_url).
--
-- db.binds: "clients" (ip;client_mac;device_mac;port, ip=2.2.2.2/4.4.4.4/5.5.5.5 -
-- метки IPTV/WIFI/YOUTUBE-пулов) и "smart" (ip;client_mac - устройства без порта)

local CREDIT_HOUR = 6      -- час, после которого лиз считается "до утра"
local TIMEOUT_WIFI = 1800  -- незарегистрированный wifi-мак (66:99:...)
local TIMEOUT_FAKE = 120   -- совсем неизвестное устройство

-- лиз INET считается до ближайших CREDIT_HOUR по местному времени, минимум 120с
local function leaseTime()
    local hour = tonumber(os.date("%H"))
    local lease
    if hour >= CREDIT_HOUR then
        lease = (24 - hour + CREDIT_HOUR) * 3600
    else
        lease = (CREDIT_HOUR - hour) * 3600
    end
    if lease < 120 then
        lease = 120
    end
    return lease
end

local function hexByte(hex, pos, len)
    if pos + len - 1 > #hex then
        return nil
    end
    return tonumber(hex:sub(pos, pos + len - 1), 16)
end

-- hex-строка -> «сырые» байты как текст (для ZTE, где значения зашиты ascii-строкой s=..;p=..)
local function hexToStr(hex)
    if #hex % 2 == 1 then
        hex = hex .. "0"
    end
    local chars = {}
    for i = 1, #hex - 1, 2 do
        chars[#chars + 1] = string.char(tonumber(hex:sub(i, i + 1), 16))
    end
    return table.concat(chars)
end

-- парсеры circuit-id по типу оборудования (db.devices.parse_type)
local circuitParsers = {}

circuitParsers["zte"] = function(circuit)
    local raw = hexToStr((circuit:gsub("^0[xX]", "")))
    local vlan = tonumber(raw:match("v=(%d+)"))
    if not vlan then
        return nil
    end
    local stack = tonumber(raw:match("s=(%d+)")) or 0
    local p = tonumber(raw:match("p=(%d+)")) or 0
    local o = tonumber(raw:match("o=(%d+)")) or 0
    return vlan, stack, stack * 100000 + p * 1000 + o
end

circuitParsers["bdcom"] = function(circuit)
    local vlan, stack, port = hexByte(circuit, 3, 4), hexByte(circuit, 9, 2), hexByte(circuit, 11, 2)
    if not vlan or not stack or not port then
        return nil
    end
    return vlan, stack, stack * 1000 + port
end

circuitParsers["dlink"] = function(circuit)
    local vlan, stack, port = hexByte(circuit, 7, 4), hexByte(circuit, 11, 2), hexByte(circuit, 13, 2)
    if not vlan or not stack or not port then
        return nil
    end
    return vlan, stack, port
end

circuitParsers["edgecore"] = function(circuit)
    local vlan, stack, port = hexByte(circuit, 7, 4), hexByte(circuit, 3, 2), hexByte(circuit, 5, 2)
    if not vlan or not stack or not port then
        return nil
    end
    return vlan, stack, port
end

local function circuitReader(circuit, parseType)
    if not circuit or circuit == "" or not parseType or parseType == "" then
        return nil
    end
    local parser = circuitParsers[parseType:lower()]
    if not parser then
        return nil
    end
    local vlan, stack, port = parser(circuit)
    if not vlan then
        return nil
    end
    return vlan, stack, port
end

function authorize(request)
    local macAbon = request.device_mac
    -- option82 может отсутствовать целиком - request.option.* тогда nil
    local macSw = request.option.remote_id or ""
    local circuitId = request.option.circuit_id or ""

    -- mac_sw может быть пустым, если в запросе нет option82
    local parseType = nil
    if macSw ~= "" then
        local device = db:getDeviceByMac(macSw)
        if device then
            parseType = device.parse_type
        end
    end

    local vlan, stack, port = circuitReader(circuitId, parseType)

    log.debug("authorize: mac=" .. macAbon .. " mac_sw=" .. macSw ..
        " parse_type=" .. tostring(parseType) .. " vlan=" .. tostring(vlan) .. " port=" .. tostring(port))

    if not vlan then
        return { error = "circuit_id parse failed: mac_sw=" .. macSw .. " parse_type=" .. tostring(parseType) .. " circuit_id=" .. circuitId }
    end

    local leaseInet = leaseTime()

    -- один запрос всех привязок на этом порту устройства (мак не задаём)
    local portBinds = db:getBind("clients", "", macSw, port)

    -- если привязка ровно одна - она и есть ответ для этого порта, независимо
    -- от того, чей мак в ней записан. Если их несколько - ищем свою по мак-адресу
    local candidate = nil
    if #portBinds == 1 then
        candidate = portBinds[1]
    elseif #portBinds > 1 then
        for _, b in ipairs(portBinds) do
            if b.client_mac == macAbon then
                candidate = b
                break
            end
        end
    end

    if candidate then
        if candidate.ip == "5.5.5.5" then
            return { pool_name = "YOUTUBE-" .. vlan, lease_time_sec = leaseInet }
        elseif candidate.ip ~= "2.2.2.2" and candidate.ip ~= "4.4.4.4" then
            return { ip_address = candidate.ip, lease_time_sec = leaseInet }
        end
    end

    -- порт целиком отдан под общий пул IPTV/WIFI - срабатывает и когда своя
    -- привязка не найдена среди нескольких на порту
    for _, b in ipairs(portBinds) do
        if b.ip == "4.4.4.4" then
            return { pool_name = "INET-" .. vlan .. "-FAKE", lease_time_sec = leaseInet }
        end
    end
    for _, b in ipairs(portBinds) do
        if b.ip == "2.2.2.2" then
            return { pool_name = "INET-" .. vlan .. "-FAKE", lease_time_sec = leaseInet }
        end
    end

    -- smart-устройство (абонентский wifi-роутер и т.п.) - привязка по мак абонента
    local smart = db:getBind("smart", macAbon)
    if #smart > 0 then
        return { ip_address = smart[1].ip, lease_time_sec = leaseInet }
    end

    -- ничего не найдено - выдаём "серый" пул
    if macAbon:match("^66:99:") then
        return { pool_name = "INET-" .. vlan .. "-WIFI", lease_time_sec = TIMEOUT_WIFI }
    end
    return { pool_name = "INET-" .. vlan .. "-FAKE", lease_time_sec = TIMEOUT_FAKE }
end
