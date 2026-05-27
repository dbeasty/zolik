-- WebSocket wrapper (requires extension-websocket dependency in game.project).
-- Native extensions expose a global `websocket` table; do not require("websocket") — bob looks for websocket.lua.

local M = {}

local function ws()
	return rawget(_G, "websocket")
end

local conn = nil
local on_message_cb = nil

function M.available()
	local api = ws()
	return api ~= nil and api.connect ~= nil
end

function M.connect(url, on_message, on_connected, on_disconnected)
	local websocket = ws()
	if not websocket then
		return false, "extension-websocket not installed"
	end
	on_message_cb = on_message
	conn = websocket.connect(url, {}, function(_, data)
		if data.event == websocket.EVENT_CONNECTED then
			if on_connected then on_connected() end
		elseif data.event == websocket.EVENT_DISCONNECTED then
			if on_disconnected then on_disconnected() end
		elseif data.event == websocket.EVENT_MESSAGE then
			local ok, msg = pcall(json.decode, data.message)
			if ok and on_message_cb then
				on_message_cb(msg)
			end
		elseif data.event == websocket.EVENT_ERROR then
			print("WS error: " .. tostring(data.message))
		end
	end)
	return true
end

function M.send(payload)
	local websocket = ws()
	if conn and websocket then
		websocket.send(conn, json.encode(payload))
	end
end

function M.disconnect()
	local websocket = ws()
	if conn and websocket then
		websocket.disconnect(conn)
		conn = nil
	end
end

return M
