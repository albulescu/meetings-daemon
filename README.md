# OMeetings

## 1 Ping
The ping process happens by pinging a websocket endpoint with following body request

```js
{
    "event" : "...",
    "time"  : "...",
    "data"  : {}
}
```

### 1.1 Events
|Event | Data | Description  |
|:--|:--|:--|
|**meeting.started**|```{"id":"...","goal":"..."}```|Call to dispatch a meeting started|
|**meeting.complete**|```{"id":"...","goal":"..."}```|Call to dispatch a meeting completed|
|**meeting.note**|```{"id":"...","goal":"...", "note":{"id":"...", "text":"...", "owner":"..."}}```|Dispatched when a note is added|
|**meeting.attachment**|```{"id":"...","goal":"...", "attachment":{"id":"...", "type":"...", "owner":"..."}}```|Dispatched when a attachment is added|