-- Подключается через script.post_auth. Вызывается вместо HTTP-запроса на postauth.addresses,
-- после того как ответ фактически отправлен NAS-у. Fire-and-forget.
function post_auth(request, response)
    log.info("post_auth: device_mac=" .. request.device_mac ..
        " ip_address=" .. response.ip_address ..
        " pool_name=" .. response.pool_name)
end
