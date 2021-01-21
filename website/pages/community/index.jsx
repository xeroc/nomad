import VerticalTextBlockList from '@hashicorp/react-vertical-text-block-list'
import SectionHeader from '@hashicorp/react-section-header'
import Head from 'next/head'

export default function CommunityPage() {
  return (
    <div id="p-community">
      <Head>
        <title key="title">Community | Nomad by HashiCorp</title>
      </Head>
      <SectionHeader
        headline="Community"
        description="Nomad is an open-source project with a thriving community where active users are willing to help you via various mediums"
        use_h1={true}
      />
      <VerticalTextBlockList
        data={[
          {
            header: 'Community Forum',
            body:
              '[Nomad Community Forum](https://discuss.hashicorp.com/c/nomad)',
          },
          {
            header: 'Bug Tracker',
            body:
              '[Issue tracker on GitHub](https://github.com/hashicorp/nomad/issues). Please only use this for reporting bugs. Do not ask for general help here; use the [Community Forum](https://discuss.hashicorp.com/c/nomad) or the mailing list for that.',
          },
          {
            header: 'Webinars',
            body:
              '[Register for webinars](https://www.hashicorp.com/events?product=nomad&type=all) or [watch recorded webinars](https://www.hashicorp.com/events/webinars/recorded?product=nomad&type=all).',
          },
          {
            header: 'Office Hours',
            body:
              '[Ask a question](https://www.hashicorp.com/community/office-hours) during community office hours',
          },
        ]}
      />
    </div>
  )
}
