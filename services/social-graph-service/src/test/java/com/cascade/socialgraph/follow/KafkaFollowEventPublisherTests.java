package com.cascade.socialgraph.follow;

import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.util.concurrent.CompletableFuture;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.kafka.core.KafkaTemplate;
import tools.jackson.databind.json.JsonMapper;

@ExtendWith(MockitoExtension.class)
class KafkaFollowEventPublisherTests {

  @Mock private KafkaTemplate<String, String> kafkaTemplate;

  @Test
  void keysEventByFolloweeId() {
    FollowCreated event = new FollowCreated(7, 42, 1234);
    String json =
        "{\"eventType\":\"FollowCreated\",\"followerId\":7,\"followeeId\":42,\"createdAtUnixMs\":1234}";
    when(kafkaTemplate.send("follow-events", "42", json))
        .thenReturn(CompletableFuture.completedFuture(null));

    new KafkaFollowEventPublisher(kafkaTemplate, JsonMapper.builder().build(), "follow-events")
        .publish(event);

    verify(kafkaTemplate).send("follow-events", "42", json);
  }
}
