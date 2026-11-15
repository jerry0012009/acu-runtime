---
name: acu-image-tools
description: Generate or edit images and read local MOV/MP4 videos using the user's configured ACU tools. Use when creating images or asking about video content, quality, references or specific video moments.
---

# Configured image service

For image generation or editing, prefer the image MCP configured through ACU Runtime. The MCP server may be called `magpie`: this is the tool entry point, not the image model's name.

- Discover `acu-image` / `generate_image` through the available MCP tool search. If the server exposes `find_tools` and `call_tool`, discover and invoke the image tool through those. A server may prefix tool names with `acu-image__` or `mcp__magpie__`.
- Leave the model argument unset to use the user's shared image model chosen in Magpie. Do not infer the image model from the conversation model or the server name.
- If generation returns a job ID, poll that same job using `generation_result`. Do not start a second generation to check progress. Save the resulting image when it is returned through a temporary URL.
- Do not silently substitute the application's built-in `ImageGen` or another image service: those may charge separate application credits. If the configured MCP is unavailable, report that and ask before changing services. Honor an explicit user request for a different service.
- Report only model and routing information actually returned by tools. MCP image generation uses the selected provider's quota; it is not free merely because it avoids application image credits.

For PDF reading and web search, continue using the application's normal file and search tools. This skill does not change command permissions or grant access to files.

# Video reading

- For local MOV/MP4 video questions, discover `acu-video-reader` and call `video_info`, then `video_watch`. This reads existing video; do not call `generate_video` to inspect it.
- The reader returns images with timestamps. Use an image-capable model or configured image interpretation tool. A text-only model must not claim to have seen the images.
- For a whole-video overview, set fps to approximately 12 divided by duration in seconds (at most 2 fps), and view_sample to 12. The default extraction has a frame cap and can otherwise cover only the beginning of a long video.
- Use `video_detail` with a narrow segment to inspect a questioned moment. Do not infer absence of defects between sampled frames, and do not promise subsecond time accuracy from second-resolution labels.
- ACU's reader is visual-only: no speech, music, lip-sync or audio-quality judgments. Never install or switch audio providers automatically.
- Distinguish source-video observations from creative recommendations. Existing output is not evidence of adherence to a newly edited prompt without generation provenance. Read-only suggestions do not authorize regeneration, edits or spending.
