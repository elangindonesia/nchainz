我是光年实验室高级招聘经理。
我在github上访问了你的开源项目，你的代码超赞。你最近有没有在看工作机会，我们在招软件开发工程师，拉钩和BOSS等招聘网站也发布了相关岗位，有公司和职位的详细信息。
我们公司在杭州，业务主要做流量增长，是很多大型互联网公司的流量顾问。公司弹性工作制，福利齐全，发展潜力大，良好的办公环境和学习氛围。
公司官网是http://www.gnlab.com,公司地址是杭州市西湖区古墩路紫金广场B座，若你感兴趣，欢迎与我联系，
电话是0571-88839161，手机号：18668131388，微信号：echo 'bGhsaGxoMTEyNAo='|base64 -D ,静待佳音。如有打扰，还请见谅，祝生活愉快工作顺利。

# N Chainz

Centralized exchanges rely on trusting that their owners will take the proper security precautions. This has led to many incidents of stolen cryptocurrency adding up to billions of dollars worth of losses, and is a stark contrast to the decentralization of the rest of the space. On the other hand, decentralized exchanges have the potential to be much more secure: theoretically, users are not vulnerable to server downtime and hacks, and can retain anonymity. 

We present **N Chainz**, a decentralized cryptocurrency exchange with a unique multi-chain architecture. We built N Chainz from the ground up, and included features such as block generation, limit orders, and the ability to trade a base token with another token. More details are included in our [whitepaper](http://github.com/RSenApps/nchainz/blob/master/NChainzWhitepaper.pdf).

## System Overview

![A system overview](images/0.png?raw=true)

## Multiple Blockchains

![A system overview](images/1.png?raw=true)

## Web UI: Order Matching & Price Chart

![A system overview](images/3.png?raw=true)

## Setup
	go get -d github.com/rsenapps/nchainz
	go install github.com/rsenapps/nchainz

## Usage
(from project root):  
``nchainz COMMAND [ARGS]``

### Account management

* ``createwallet``  
Create a wallet with a pair of keys  
* ``getbalance ADDRESS SYMBOL``  
Get the balance for an address  
* ``printaddresses``  
Print all adddreses in wallet file  

### Creating transactions

* ``order BUY_AMT BUY_SYMBOL SELL_AMT SELL_SYMBOL ADDRESS``  
Create an ORDER transaction
* ``transfer AMT SYMBOL FROM TO``  
Create a TRANSFER transaction
* ``freeze AMT SYMBOL FROM UNFREEZE_BLOCK``  
Create a FREEZE tokens transaction
* ``cancel SYMBOL ORDER_ID``  
Create a CANCEL_ORDER transaction
* ``claim AMT SYMBOL ADDRESS``  
Create a CLAIM_FUNDS transaction
* ``create SYMBOL SUPPLY DECIMALS ADDRESS``  
Create a CREATE_TOKEN transaction

### Running a node or miner

* ``node HOSTNAME:PORT``  
  Start up a full node providing your hostname on the given port
* ``printchain DB SYMBOL``  
  Prints all the blocks in the blockchain
* ``webserver PORT``  
  Run a webserver on the given port
  
## Authors

* [Ryan Senanayake](http://rsenapps.com/)
* [Nicholas Egan](http://nicholasegan.me/)
* [Elizabeth Wei](http://lizziew.github.io/)

## License
The GNU Affero General Public License (see LICENSE)
