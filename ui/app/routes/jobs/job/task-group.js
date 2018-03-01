import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchRecord, watchRelationship } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default Route.extend(WithWatchers, {
  model({ name }) {
    // If the job is a partial (from the list request) it won't have task
    // groups. Reload the job to ensure task groups are present.
    return this.modelFor('jobs.job')
      .reload()
      .then(job => {
        return job
          .hasMany('allocations')
          .reload()
          .then(() => {
            return job.get('taskGroups').findBy('name', name);
          });
      });
  },

  setupController(controller, model) {
    const job = model.get('job');
    controller.set('watchers', {
      job: this.get('watchJob').perform(job),
      summary: this.get('watchSummary').perform(job),
      allocations: this.get('watchAllocations').perform(job),
    });
    return this._super(...arguments);
  },

  watchJob: watchRecord('job'),
  watchSummary: watchRelationship('summary'),
  watchAllocations: watchRelationship('allocations'),

  watchers: collect('watchJob', 'watchSummary', 'watchAllocations'),
});
