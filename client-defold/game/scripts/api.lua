-- REST helpers for lobby / auth (shared by GUI).

local M = {}

M.base_url = "http://127.0.0.1:8090"
M.token = nil
M.player_id = nil

local function auth_headers()
	local h = { ["Content-Type"] = "application/json" }
	if M.token then
		h["Authorization"] = "Bearer " .. M.token
	end
	return h
end

function M.http_json(path, method, body, cb)
	local headers = auth_headers()
	local payload = body and json.encode(body) or nil
	http.request(M.base_url .. path, method, function(_, _, response)
		if response.status >= 200 and response.status < 300 then
			local ok, data = pcall(json.decode, response.response)
			if ok then
				cb(nil, data)
			else
				cb("bad json", nil)
			end
		else
			cb(response.response or ("HTTP " .. tostring(response.status)), nil)
		end
	end, headers, payload)
end

function M.guest_login(name, cb)
	M.http_json("/auth/guest", "POST", { guestName = name or "Player" }, function(err, data)
		if err then
			cb(err)
			return
		end
		M.token = data.accessToken
		M.player_id = data.userId or M.token_subject()
		cb(nil, data)
	end)
end

function M.create_game(cb)
	M.http_json("/games", "POST", { initialMeldMinimum = 35 }, function(err, data)
		if err then
			cb(err)
			return
		end
		cb(nil, data)
	end)
end

function M.join_game(id_or_code, cb)
	M.http_json("/games/" .. id_or_code .. "/join", "POST", {}, function(err, data)
		if err then
			cb(err)
			return
		end
		cb(nil, data)
	end)
end

function M.start_game(game_id, cb)
	M.http_json("/games/" .. game_id .. "/start", "POST", {}, function(err, data)
		if err then
			cb(err)
			return
		end
		cb(nil, data)
	end)
end

function M.add_ai(game_id, difficulty, cb)
	M.http_json("/games/" .. game_id .. "/add-ai", "POST", { difficulty = difficulty or "medium" }, function(err, data)
		if err then
			cb(err)
			return
		end
		cb(nil, data)
	end)
end

function M.token_subject()
	if not M.token then
		return nil
	end
	local payload = M.token:match("%.(.-)%.")
	if not payload then
		return nil
	end
	payload = payload:gsub("-", "+"):gsub("_", "/")
	while #payload % 4 ~= 0 do
		payload = payload .. "="
	end
	local raw = crypt.base64decode(payload)
	if not raw then
		return nil
	end
	local ok, data = pcall(json.decode, raw)
	if ok and data.sub then
		return data.sub
	end
	return nil
end

function M.ws_url(game_id)
	local base = M.base_url
	base = base:gsub("^http", "ws")
	return base .. "/ws/games/" .. game_id .. "?token=" .. (M.token or "")
end

return M
