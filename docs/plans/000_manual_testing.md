# Updates Needed After Manual Testing

This document contains findings from manual testing that are less than ideal (think user confusion or bad UX) or outright bugs.

## Bad UX

### No context as tho what is needod or missing

```shell
./axe agents init
accepts 1 arg(s), received 0
```

```shell
./axe agents show
accepts 1 arg(s), received 0
```

```shell
./axe run
accepts 1 arg(s), received 0
```

## Bugs

### Non-Intuitive skill reference

This config entry should automatically resolve to the skills directory but doesn't:
`skill = "yti"`

and `skill = "skills/yti" does:
```shell
echo "https://www.youtube.com/watch?v=f6F8GtIY3ZE" | axe run yti
failed to read skill: read /home/jaron/.config/axe/skills/yti: is a director
```
had to do this which is not intuitive:
`skill = "skills/yti/SKILL.md"`

### Multi-Turn Needed?
```shell
echo "https://www.youtube.com/watch?v=f6F8GtIY3ZE" | axe run yti
I'll fetch the transcript for that YouTube video.

```bash
scripts/get_transcript.sh "f6F8GtIY3ZE" "text"
```

Let me retrieve this for you using the TranscriptAPI.

**Note:** I need to make the actual API call. Let me fetch the transcript directly:

```
GET https://transcriptapi.com/api/v2/youtube/transcript
Parameters:
  - video_url: f6F8GtIY3ZE
  - format: text
```

I don't have direct API access in this environment, but here's what I can do:

1. **Provide you the command** to run locally:
   ```bash
   scripts/get_transcript.sh "https://www.youtube.com/watch?v=f6F8GtIY3ZE"
   ```

2. **Or use curl directly**:
   ```bash
   curl -H "Authorization: Bearer $TRANSCRIPT_API_KEY" \
     "https://transcriptapi.com/api/v2/youtube/transcript?video_url=f6F8GtIY3ZE&format=text"
   ```

Could you either:
- Run one of these commands and share the output, or
- Tell me the **video title** so I know what we're working with?

Once I have the transcript, I'll convert it into a clean, readable markdown summary and save it to `docs/summaries/`.%

```
