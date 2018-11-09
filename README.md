Go-LogTransport
---

This package conveniently produces logs to be consumed by [`go-logsink`][0].

See **[Go Docs][1]** for usage instructions.

---

#### Sample logs:

```
2018/11/09 20:35:50 INFO: 0: --> TESTSVC: 5d13417d-a759-45a3-948c-40f62d2e5e11: test-svcaction: test-description
2018/11/09 20:35:53 INFO: 0: --> TESTSVC: test-eventaction: test-svcaction: test-log
2018/11/09 20:35:53 INFO: 0: --> SOME-NAME: 6678d0ee-f38e-43da-9b10-1e872ce92c36: test-svcaction: 6678d0ee-f38e-43da-9b10-1e872ce92c36
2018/11/09 20:35:53 ERROR: 0: --> SOME-NAME: 8b63e749-825a-4b03-8cd1-87719658f76e: test-svcaction: 8b63e749-825a-4b03-8cd1-87719658f76e
2018/11/09 20:36:38 INFO: 0: --> TESTSVC: : : C:/Users/jaska/Documents/GoProjects/src/github.com/TerrexTech/go-logtransport/log/log_suite_test.go:428: ===> test==========
========================
2018/11/09 20:36:38 DEBUG: 0: --> SOME-NAME: e457fd19-684d-4817-b587-2664f6672e4a: test-svcaction:
2018/11/09 15:36:38 C:/Users/jaska/Documents/GoProjects/src/github.com/TerrexTech/go-logtransport/log/log_suite_test.go:431: ===> e457fd19-684d-4817-b587-2664f6672e4a
========================
--------------
==> Data 0: model.Event:
{"aggregateID":0,"correlationID":"00000000-0000-0000-0000-000000000000","data":{"aggregateID":1,"correlationID":"00000000-0000-0000-0000-000000000000","uuid":"00000000-0000-0000-0000-000000000000"},"eventAction":"test-action","nanoTime":0,"serviceAction":"","userUUID":"00000000-0000-0000-0000-000000000000","uuid":"00000000-0000-0000-0000-000000000000","version":0,"yearBucket":0}
--------------
==> Data 1: model.EventStoreQuery:
{"aggregateID":1,"aggregateVersion":3,"correlationID":"00000000-0000-0000-0000-000000000000","uuid":"00000000-0000-0000-0000-000000000000"}
--------------
==> Data 2: []model.EventMeta:
-----
=> Index 0: EventMeta:
{"aggregateID":1,"aggregateVersion":3}
-----
=> Index 1: EventMeta:
{"aggregateID":2,"aggregateVersion":8}
-----
--------------
==> Data 3: model.KafkaResponse:
{"aggregateID":1,"correlationID":"00000000-0000-0000-0000-000000000000","error":"","errorCode":0,"eventAction":"testaction","input":"","result":{"aggregateID":1,"correlationID":"00000000-0000-0000-0000-000000000000","uuid":"00000000-0000-0000-0000-000000000000"},"serviceAction":"","topic":"","uuid":"00000000-0000-0000-0000-000000000000"}
--------------
==> Data 4: testData5
--------------
==> Data 5: 4
--------------
========================
```

  [0]: https://github.com/TerrexTech/go-logsink
  [1]: https://godoc.org/github.com/TerrexTech/go-logtransport/log
