# FeedMessageProcessor

**File:** `processors/transit/feed_message_events/feed_message_event_processor.go`
**Stateful:** No

## Input

Raw bytes (`[]uint8`) or a pre-decoded `*gtfs.FeedMessage`.

## Processing

1. Type-switches on the incoming value.
2. If raw bytes, deserializes using `proto.Unmarshal` into a `*gtfs.FeedMessage`. Messages that fail to deserialize are dropped.
3. Wraps the decoded feed with the configured agency ID.

## Output

`*pb.FeedMessageEvent`

```proto
message FeedMessageEvent {
  string agency_id = 1;
  transit_realtime.FeedMessage feed_message = 2;
}
```
