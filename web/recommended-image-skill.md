---
name: acu-image-tools
description: Generate or edit images using the user's configured ACU/Magpie MCP image service. Use for requests to draw, create an image, generate an illustration, or edit an image.
---

# Configured image service

For image generation or editing, prefer the image MCP configured through ACU Runtime. The MCP server may be called `magpie`: this is the tool entry point, not the image model's name.

- Discover `acu-image` / `generate_image` through the available MCP tool search. If the server exposes `find_tools` and `call_tool`, discover and invoke the image tool through those. A server may prefix tool names with `acu-image__` or `mcp__magpie__`.
- Leave the model argument unset to use the user's shared image model chosen in Magpie. Do not infer the image model from the conversation model or the server name.
- If generation returns a job ID, poll that same job using `generation_result`. Do not start a second generation to check progress. Save the resulting image when it is returned through a temporary URL.
- Do not silently substitute the application's built-in `ImageGen` or another image service: those may charge separate application credits. If the configured MCP is unavailable, report that and ask before changing services. Honor an explicit user request for a different service.
- Report only model and routing information actually returned by tools. MCP image generation uses the selected provider's quota; it is not free merely because it avoids application image credits.

For PDF reading and web search, continue using the application's normal file and search tools. This skill does not change command permissions or grant access to files.
