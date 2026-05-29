local function stringify(inlines)
  return pandoc.utils.stringify(inlines)
end

local function parse_marker(block)
  if block.t ~= "Para" then
    return nil
  end

  local text = stringify(block.content)
  local rest = nil
  if text:sub(1, 3) == "!!!" then
    rest = text:sub(4)
  elseif text:sub(1, 3) == "???" then
    rest = text:sub(4)
    if rest:sub(1, 1) == "+" then
      rest = rest:sub(2)
    end
  else
    return nil
  end

  local kind, title = rest:match("^%s*([%w_-]+)%s*(.*)$")
  if kind == nil then
    return nil
  end

  title = title or ""
  title = title:gsub("^%s+", ""):gsub("%s+$", "")
  title = title:gsub('^"(.*)"$', "%1")
  title = title:gsub("^'(.*)'$", "%1")
  title = title:gsub("^“(.*)”$", "%1")
  title = title:gsub("^‘(.*)’$", "%1")
  if title == "" then
    title = kind
  end

  return {
    kind = kind,
    title = title,
  }
end

local function parse_body(block)
  if block == nil or block.t ~= "CodeBlock" then
    return nil
  end

  local parsed = pandoc.read(block.text, "markdown")
  return parsed.blocks
end

function Pandoc(doc)
  local result = pandoc.List()
  local i = 1
  while i <= #doc.blocks do
    local marker = parse_marker(doc.blocks[i])
    if marker ~= nil then
      local body = parse_body(doc.blocks[i + 1])
      if body ~= nil then
        local label = string.upper(marker.kind) .. ": " .. marker.title
        result:insert(pandoc.Para({ pandoc.Str(label) }))
        for _, body_block in ipairs(body) do
          result:insert(body_block)
        end
        i = i + 2
      else
        result:insert(doc.blocks[i])
        i = i + 1
      end
    else
      result:insert(doc.blocks[i])
      i = i + 1
    end
  end
  doc.blocks = result
  return doc
end
