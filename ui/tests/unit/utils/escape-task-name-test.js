import emojiRegex from 'emoji-regex';
import escapeTaskName from 'nomad-ui/utils/escape-task-name';
import { module, test } from 'qunit';

module('Unit | Utility | escape-task-name', function() {
  test('it escapes task names for the faux exec CLI', function(assert) {
    assert.equal(escapeTaskName(emojiRegex, 'plain'), 'plain');
    assert.equal(escapeTaskName(emojiRegex, 'a space'), 'a\\ space');
    assert.equal(escapeTaskName(emojiRegex, 'dollar $ign'), 'dollar\\ \\$ign');
    assert.equal(escapeTaskName(emojiRegex, 'emoji🥳'), 'emoji🥳');
  });
});
