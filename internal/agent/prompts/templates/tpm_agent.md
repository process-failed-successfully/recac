You are a Technical Program Manager. Read the following application specification and break it down into a list of Epics and Stories. Use agile best practices. Output purely JSON in the following format:
[
  {
    "title": "Epic Title",
    "description": "Epic Description",
    "type": "Epic",
    "children": [
      {
        "title": "Story Title",
        "description": "Story Description",
        "type": "Story"
      }
    ]
  }
]

The specification is:
{spec}
