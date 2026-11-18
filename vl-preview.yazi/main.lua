--- @since 26.9.1

local M = {}

function M:peek(job)
	if job.area.w == 0 or job.area.h == 0 then
		return
	end
	local url = job.file.url
	if not url.spec.is_regular then
		return require("code"):peek(job)
	end

	local output, err = Command("vl"):arg({
		"preview",
		"--color=always",
		"--skip=" .. job.skip,
		"--lines=" .. job.area.h,
		"--width=" .. job.area.w,
		"--tab-size=" .. math.max(1, rt.preview.tab_size),
		"--", tostring(url.path),
	}):output()

	if not output then
		return require("empty").msg(job, "Could not run vl preview: " .. tostring(err))
	end
	if not output.status.success then
		local message = output.stderr:gsub("%s+$", "")
		return require("empty").msg(job, message ~= "" and message or "vl preview failed")
	end
	if job.skip > 0 and output.stdout == "" then
		return require("code"):peek(job)
	end
	ya.preview_widget(job, ui.Text.parse(output.stdout):area(job.area))
end

function M:seek(job)
	return require("code"):seek(job)
end

return M
