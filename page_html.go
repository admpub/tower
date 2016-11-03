package main

var defaultPageHTML = `<html>
  <head>
    <style>
      *{
        font-family: Helvetica Neue, Arial, Verdana, sans-serif;
      }
      body{
        margin: 0;
      }
      .header{
        width:100%;
        height: 70px;
        background-color: #D8E5F2;
      }
      h1{
        font-size: 30px;
        line-height: 70px;
        width: 880px;
        margin: 0 auto;
        padding-left: 20px;
      }
      .content{
        width: 880px;
        margin: 0 auto;
        padding-left:20px;
      }

      h2{
        font-size:20px;
      }

      .message{
        margin: 40px 0 60px 0;
      }

      .snippet, .trace{
        margin-left: -15px;
        padding:14px;
        border: 1px solid #D8E5F2;
        border-radius: 5px;
        -moz-border-radius: 5px;
        -webkit-border-radius: 5px;
        margin-bottom: 30px;
      }

      .numbers, .codes{
        line-height: 22px;
      }
      .numbers{
        float:left;
        text-align: right;
        margin-right: 15px;
        color: #929292;
      }

      .trace ul{
        padding: 0;
        margin: 0;
        list-style: none;
      }
      .trace ul li{
        margin-bottom: 10px;
      }

      .trace .func{
        color: #929292;
      }

      .clearfix{
        clear: both;
      }

    </style>
  </head>
  <body>
    <div class="header">
      <h1>{{.Title}} -- {{.Time}}</h1>
    </div>

    <div class="content">
      <div class="message">
        {{.Message}}
      </div>


      {{if .ShowSnippet}}
      <h2>{{.SnippetPath}}</h2>
      <div class="snippet">
        <div class="numbers">
          {{range .Snippet}}
            {{if .Current}}
              <strong>{{.Number}}</strong>
            {{else}}
              {{.Number}}
            {{end}}
            <br/>
          {{end}}
        </div>

        <div class="codes">
          {{range .Snippet}}
            {{if .Current}}
              <strong>{{.Code}}</strong>
            {{else}}
              {{.Code}}
            {{end}}
            <br/>
          {{end}}
        </div>
        <div class="clearfix"></div>
      </div>
      {{end}}


      {{if .ShowTrace}}
      <h2>Trace</h2>
      <div class="trace">
        <ul>
          {{range .Trace}}
          <li>
            {{if .AppFile}}
              <strong>{{.File}}</strong>
            {{end}}
            {{if not .AppFile}}
              {{.File}}
            {{end}}
            <br/>
            <span class="func">{{.Func}}</span>
          </li>
          {{end}}
        </ul>
      </div>
      {{end}}
    </div>
  </body>
</html>
`
